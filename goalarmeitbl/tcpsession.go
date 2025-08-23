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
    to_send chan []byte
    conn *net.TCPConn
    wg sync.WaitGroup
    stoponce sync.Once
    recvbuffersize int
}

// Creates new TCPSession. Indirectly invoked by TCPServer and TCPClient
// Connection release is indicated by Events channel closure.
//
// User must handle the following Events:
// "Recv" + []byte: data received
// "Sent" + int: data chunk sent, "n" chunks to go
// "RecvEof": connection closed in rx direction
// "SendEof": connection closed in tx direction
// "Err": error, connection no longer valid (no need to call Close() to release it).
//        This event may happen twice. Call Close() on the first one to avoid this.
//
// API: Send() and Close(). Should not be called before Start(), which is normally
// called by TCPServer and TCPClient.

func NewTCPSession() *TCPSession {
    // FIXME allow configuration of queue depths for high-throughput applications
    // FIXME allow configuration of recv buffer size
    send_queue_depth := 2

    h := new(TCPSession)
    // rationale for +2: Err from send goroutine + Err from recv goroutine
    h.Events = make(chan Event, send_queue_depth + 2)
    // rationale for +1: nil from stop()
    h.to_send = make(chan []byte, send_queue_depth + 1)
    h.recvbuffersize = 1500

    return h
}

// Once a session is started, must be released by calling Bye()
func (h *TCPSession) Start(conn *net.TCPConn) {
    h.conn = conn
    h.wg = sync.WaitGroup{}
    h.wg.Add(2)

    go func() {
        h.wg.Wait()
        h.conn.Close() // just to make sure
        close(h.Events) // disengage user 
        log.Printf("TCPSession %p: exited -------------", h)
    }()

    go h.recv()
    go h.send()
    log.Printf("TCPSession %p ==================", h)
}

// Data receiving goroutine. Stopped by closure of h.conn
func (h *TCPSession) recv() {
    for {
        data := make([]byte, h.recvbuffersize)
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

    h.stoponce.Do(func() {
        h.conn.Close()
        for len(h.to_send) > 0 {
            <-h.to_send
        }
        h.to_send <- nil
        did_stop = true 
        log.Printf("TCPSession %p: stop()", h)
    })

    return did_stop
}

// Data sending goroutine. Stopped by nil msg in h.to_send
func (h *TCPSession) send() {
    is_open := true

    for {
        data := <-h.to_send

        if data == nil {
            break
        }

        if !is_open {
            continue
        }

        if len(data) == 0 {
            log.Printf("TCPSession %p: gosend: shutdown", h)
            h.conn.CloseWrite()
            is_open = false
            continue
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
                is_open = false
                break
            }

            log.Printf("TCPSession %p: gosend: sent %d", h, n)
            data = data[n:]
        }

        if is_open {
            h.Events <- Event{"Sent", len(h.to_send)}
        }
    }

    log.Printf("TCPSession %p: gosend: exited", h)
    h.wg.Done()
}

// Public interface

// Send data
// empty slice = shutdown connection for sending
// Warning: the send queue channel has limited length and may block if called several times in succession.
// Listen for the "Sent" event to throttle
// Should not call Send() after Close() -- may block forever
func (h *TCPSession) Send(data []byte) {
    log.Printf("TCPSession %p: Send %d", h, len(data))
    if data == nil {
        // nil has other meaning
        data = []byte{}
    }
    h.to_send <-data
}

// Close connection
// It is guaranteed that no events will be emitted afterwards
func (h *TCPSession) Close() {
    log.Printf("TCPSession %p: Close", h)
    h.stop()
    // drain all outstanding events until h.Events closure
    for evt := range h.Events {
        log.Printf("TCPSession %p: drained %s", h, evt.Name)
    }
}
