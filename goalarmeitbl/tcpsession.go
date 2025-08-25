package goalarmeitbl

import (
    "net"
    "io"
    "log"
    "sync"
    "time"
)

type tcpsessionevent struct {
    name string
    data []byte
}

type TCPSession struct {
    Events chan Event
    queue_depth int

    conn *net.TCPConn
    recv_buffer_size int

    to_send chan []byte         // send queue
    send_queue_depth int
    send_sem chan struct{}      // semaphore to protect Send() against closed to_send

    wg sync.WaitGroup           // check all goroutines have stopped
    stoponce sync.Once          // make sure stop() acts only once

    timeouts map[*Timeout]bool  // Timeouts associated with this session
    timeouts_sem chan struct {} // and its semaphore
}

// Creates new TCPSession. Indirectly invoked by TCPServer and TCPClient
// Connection release is indicated by Events channel closure, which only
// happens when user calls Close().
//
// User must handle the following Events:
// "Recv" + []byte: data received
// "Sent" + int: data chunk sent, "n" chunks to go
// "RecvEof": connection closed in rx direction
// "SendEof": connection closed in tx direction
// "Err": error, connection no longer valid (must call Close() to release it)
//
// API: Send() and Close(). Should not be called before Start(), which is normally
// called by TCPServer and TCPClient.
// Timeout API: Timeout() to create timeouts owned by this session

func NewTCPSession() *TCPSession {
    // FIXME allow configuration of queue depths for high-throughput applications
    // FIXME allow configuration of recv buffer size

    h := new(TCPSession)
    // rationale for +1: "Sent" events + at least one "Recv"/error event
    h.queue_depth = 1
    h.send_queue_depth = 2
    if h.send_queue_depth <= 0 {
        h.send_queue_depth = 1
    }
    h.recv_buffer_size = 1500
    h.Events = make(chan Event, h.send_queue_depth + h.queue_depth)
    h.to_send = make(chan []byte, h.send_queue_depth)
    h.send_sem = make(chan struct{}, 1)
    h.send_sem <-struct{}{}

    h.timeouts = make(map[*Timeout]bool)
    h.timeouts_sem = make(chan struct{}, 1)
    h.timeouts_sem <-struct{}{}

    h.wg = sync.WaitGroup{}
    h.conn = nil

    return h
}

func (h *TCPSession) Start(conn *net.TCPConn) {
    h.conn = conn

    h.wg.Add(3)
    go h.recv()
    go h.send()

    // Teardown when both goroutines are finished and users calls Close()
    go func() {
        h.wg.Wait()
        // Close() is still running and draining h.Events at this point, so if any timeout triggers,
        // it won't block on h.Events
        h.release_timeouts()
        close(h.Events) // disengage user
        log.Printf("TCPSession %p: exited -------------", h)
    }()

    log.Printf("TCPSession %p ==================", h)
}

// Data receiving goroutine. Stopped by closure of h.conn
func (h *TCPSession) recv() {
    for {
        data := make([]byte, h.recv_buffer_size)
        n, err := h.conn.Read(data)
        if err != nil {
            if err == io.EOF {
                log.Printf("TCPSession %p: gorecv: eof", h)
                h.Events <- Event{"RecvEof", nil}
            } else {
                log.Printf("TCPSession %p: gorecv: err or stop", h)
                if h.stop() {
                    h.Events <- Event{"Err", nil}
                }
            }
            break // exit goroutine
        }
        log.Printf("TCPSession %p: gorecv: received %d", h, n)
        h.Events <- Event{"Recv", data[:n]}
    }

    log.Printf("TCPSession %p: gorecv: exited", h)
    h.wg.Done()
}

// indirectly stops goroutines
func (h *TCPSession) stop() bool {
    did_stop := false

    // necessary since h.to_send must not be closed more than once
    h.stoponce.Do(func() {
        // indirectly stops recv goroutine, if running
        if h.conn != nil {
            h.conn.Close()
        }

        // Protect against race with Send()
        <-h.send_sem
        // makes sure further Send() goes to /dev/null
        h.send_queue_depth = 0
        // indirectly stops send goroutine, if running
        close(h.to_send)
        h.send_sem <-struct{}{}

        did_stop = true 
        log.Printf("TCPSession %p: stop()", h)
    })

    return did_stop
}

// Data sending goroutine. Stopped by closing channel h.to_send
func (h *TCPSession) send() {
loop:
    for data := range h.to_send {
        if len(data) == 0 {
            log.Printf("TCPSession %p: gosend: shutdown", h)
            h.conn.CloseWrite()
            break loop
        }

        for len(data) > 0 {
            log.Printf("TCPSession %p: gosend: sending %d", h, len(data))
            n, err := h.conn.Write(data)

            if err != nil {
                if err == io.EOF {
                    log.Printf("TCPSession %p: gosend: eof", h)
                    h.Events <- Event{"SendEof", nil}
                } else {
                    log.Printf("TCPSession %p: gosend: err", h)
                    if h.stop() {
                        h.Events <- Event{"Err", nil}
                    }
                }
                break loop
            }

            log.Printf("TCPSession %p: gosend: sent %d", h, n)
            data = data[n:]
        }

        h.Events <- Event{"Sent", len(h.to_send)}
    }

    // Drain channel until closure
    for range h.to_send {
    }

    log.Printf("TCPSession %p: gosend: exited", h)
    h.wg.Done()
}

// Public interface

// Send data
// empty slice = shutdown connection for sending
// Returns true if successfully queued, false if queue is full 
// Send after close does not block, does not panic, and returns true
// Listen for the "Sent" event to manage the queue and avoid queue-full failures
func (h *TCPSession) Send(data []byte) bool {
    log.Printf("TCPSession %p: Send %d", h, len(data))

    // Protect against race with stop()
    <-h.send_sem
    defer func() {
        h.send_sem <-struct{}{}
    }()

    // protection against closed h.to_send
    if h.send_queue_depth > 0 {
        select {
        case h.to_send <-data:
            return true
        default:
            log.Printf("TCPSession %p: Send() would block", h)
            return false
        }
    }

    log.Printf("TCPSession %p: Send() after close", h)
    return true
}

// Close connection and release resources
// No events will be emitted after this call returns
func (h *TCPSession) Close() {
    log.Printf("TCPSession %p: Close", h)
    h.stop()
    h.wg.Done()
    // Blocks until h.Events is closed by main session' own goroutine
    for evt := range h.Events {
        log.Printf("TCPSession %p: drained %s", h, evt.Name)
    }
}

// Should not be called by user. This is called by the owned Timeout upon Timeout.Free()
func (h *TCPSession) ReleaseTimeout(to *Timeout) {
    <-h.timeouts_sem
    if _, ok := h.timeouts[to]; ok {
        log.Printf("TCPSession %p: released timeout %p", h, to)
        delete(h.timeouts, to)
    }
    h.timeouts_sem <-struct{}{}
}

// Create new Timeout owned by this session
// (meaning it is automatically stopped and released when the session is closed)
func (h *TCPSession) Timeout(avgto time.Duration, fudge time.Duration, cbchmsg string) (*Timeout) {
    to := NewTimeout(avgto, fudge, h.Events, cbchmsg, h)
    <-h.timeouts_sem
    h.timeouts[to] = true
    h.timeouts_sem <-struct{}{}
    log.Printf("TCPSession %p: new owned timeout %p", h, to)
    
    return to
}

// Release all owned timeouts upon session closure
func (h *TCPSession) release_timeouts() {
    for {
        var to *Timeout

        // Get some owned timeout
        <-h.timeouts_sem
	    for k := range h.timeouts {
		    to = k
		    break
	    }
        h.timeouts_sem <-struct{}{}

        if to == nil {
            break
        }

        // synchronously calls ReleaseTimeout() and prevents further timeout events
        to.Free()
    }
}
