package goalarmeitbl

import (
    "net"
    "io"
    "log"
    "sync"
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
}

// Creates new TCPSession. Indirectly invoked by TCPServer and TCPClient
// Connection release is indicated by Events channel closure.
//
// User must handle the following Events:
// "Recv" + []byte: data received
// "Sent" + int: data chunk sent, "n" chunks to go
// "RecvEof": connection closed in rx direction
// "SendEof": connection closed in tx direction
// "Err": error, connection no longer valid (no need to call Close() to release it)
//
// API: Send() and Close(). Should not be called before Start(), which is normally
// called by TCPServer and TCPClient.

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

    return h
}

func (h *TCPSession) Start(conn *net.TCPConn) {
    h.conn = conn

    h.wg = sync.WaitGroup{}
    h.wg.Add(2)
    go h.recv()
    go h.send()

    // Teardown when both goroutines are finished
    go func() {
        h.wg.Wait()
        h.conn.Close()
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
            // exit goroutine
            break
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
        // indirectly stops recv goroutine
        h.conn.Close()

        <-h.send_sem
        // makes sure further Send() goes to /dev/null
        h.send_queue_depth = 0
        // indirectly stops send goroutine
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

// Close connection
// It is guaranteed that no events will be emitted afterwards
func (h *TCPSession) Close() {
    log.Printf("TCPSession %p: Close", h)
    h.stop()
    // drain all outstanding events until h.Events closure
    // (does not panic if h.Events is closed already)
    for evt := range h.Events {
        log.Printf("TCPSession %p: drained %s", h, evt.Name)
    }
}
