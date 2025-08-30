package goalarmeitbl

import (
    "net"
    "io"
    "log"
    "time"
)

type tcpsessionevent struct {
    name string
    data []byte
}

type TCPSessionOwner interface {
    Closed(*TCPSession)
}

type TCPSession struct {
    owner TCPSessionOwner

    Events chan Event
    queue_depth int

    conn *net.TCPConn
    recv_buffer_size int

    to_send chan []byte         // send queue
    send_queue_depth int

    waitgroup chan struct{}    // check all goroutines have stopped

    timeouts *TimeoutOwner     // Timeouts associated with this session
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
// "Err": error, connection no longer valid (still must call Close() to release it)
//
// API: Send() and Close(). Should not be called before Start(), but this is normally
// called by TCPServer and TCPClient.
// Timeout API: Timeout() to create timeouts owned by this session
// All APIs must not be called after Close()

func NewTCPSession(owner TCPSessionOwner) *TCPSession {
    // FIXME allow configuration of queue depths for high-throughput applications
    // FIXME allow configuration of recv buffer size

    h := new(TCPSession)
    h.owner = owner
    // rationale for +1: "Sent" events + at least one "Recv"/error event
    h.queue_depth = 1
    h.send_queue_depth = 2
    if h.send_queue_depth <= 0 {
        h.send_queue_depth = 1
    }
    h.recv_buffer_size = 1500
    h.Events = make(chan Event, h.send_queue_depth + h.queue_depth)
    h.to_send = make(chan []byte, h.send_queue_depth)

    h.timeouts = NewTimeoutOwner(h.Events)

    // buffer must be >= 1 to avoid deadlock if Close() called upon unstarted session
    h.waitgroup = make(chan struct{}, 1)

    h.conn = nil

    return h
}

func (h *TCPSession) Start(conn *net.TCPConn) {
    h.conn = conn
    h.Events <- Event{"Connected", nil}

    go h.recv()
    go h.send()

    // "deferred" teardown, only for sessions that have been Start()ed
    go func() {
        // wait on Close(), go send() and go recv()
        <-h.waitgroup
        <-h.waitgroup
        <-h.waitgroup

        // Close() is still running and draining h.Events at this point,
        // so if any timeout triggers meanwhile, it won't block on h.Events
        h.timeouts.Release()

        // Disengage user and ongoing Close()
        close(h.Events)

        // Notify owner e.g. TCPServer
        h.owner.Closed(h)

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
                h.Events <- Event{"Err", nil}
            }
            break // exit goroutine
        }
        log.Printf("TCPSession %p: gorecv: received %d", h, n)
        h.Events <- Event{"Recv", data[:n]}
    }

    // Signals session teardown goroutine
    h.waitgroup <- struct{}{}

    log.Printf("TCPSession %p: gorecv: exited", h)
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
                    h.Events <- Event{"Err", nil}
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

    // Signals session teardown goroutine
    h.waitgroup <-struct{}{}

    log.Printf("TCPSession %p: gosend: exited", h)
}

// Public interface

// Send data
// empty slice = shutdown connection for sending
// May block if send queue is full. Session should have send queue deep enough for the use case
// Listen for the "Sent" event to manage the queue
func (h *TCPSession) Send(data []byte) {
    log.Printf("TCPSession %p: Send %d", h, len(data))
    h.to_send <-data
}

// Close connection and release resources
// No events will be emitted after this call returns
func (h *TCPSession) Close() {
    log.Printf("TCPSession %p: Closing...", h)

    // indirectly stops recv goroutine, if running
    if h.conn != nil {
        h.conn.Close()
    }

    // indirectly stops send goroutine, if running
    close(h.to_send)

    // Signals session teardown goroutine, if running
    h.waitgroup <-struct{}{}

    // Blocks until h.Events is closed by main session' own goroutine
    for evt := range h.Events {
        log.Printf("TCPSession %p: drained %s", h, evt.Name)
    }
}

// Create new Timeout owned by this session
func (h *TCPSession) Timeout(avgto time.Duration, fudge time.Duration, cbchmsg string) (*Timeout) {
    to := h.timeouts.Timeout(avgto, fudge, cbchmsg)
    log.Printf("TCPSession %p: new owned timeout %p", h, to)
    return to
}
