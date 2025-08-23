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
}

func NewTCPSession() *TCPSession {
    // FIXME allow configuration of connection timeout
    // FIXME allow configuration of queue depths for high-throughput applications
    send_queue_depth := 2
    // rationale: recverr + senderr in case of unexpected close
    minimum_depth := 2

    h := new(TCPSession)
    h.Events = make(chan Event, send_queue_depth + minimum_depth)
    h.to_send = make(chan []byte, send_queue_depth)

    return h
}

// Once a session is started, must be released by calling Bye()
func (h *TCPSession) Start(conn *net.TCPConn) {
    h.conn = conn
    h.wg = sync.WaitGroup{}
    h.wg.Add(2)

    go func() {
        h.wg.Wait()
        h.conn.Close()
        close(h.Events) // user disengages
        log.Printf("TCPSession %p: stopped -------------", h)
    }()

    go h.recv()
    go h.send()
    log.Printf("TCPSession %p ==================", h)
}

// Data receiving goroutine. Stopped by closure of h.conn
func (h *TCPSession) recv() {
    for {
        data := make([]byte, 1500)
        n, err := h.conn.Read(data)
        if err != nil {
            if err == io.EOF {
                log.Printf("TCPSession %p: gorecv: eof", h)
                h.Events <- Event{"RecvEof", nil}
            } else {
                log.Printf("TCPSession %p: gorecv: err or stop", h)
                h.Events <- Event{"Err", nil}
                h.conn.Close()
            }
            // exit goroutine
            break
        }
        log.Printf("TCPSession %p: gorecv: received %d", h, n)
        h.Events <- Event{"Recv", data[:n]}
    }

    h.wg.Done()
    log.Printf("TCPSession %p: gorecv: stopped", h)
}

// Data sending goroutine. Stopped by closure of h.conn or closure of channel h.to_send
func (h *TCPSession) send() {
    is_open := true

    for data := range h.to_send {
        if data == nil {
            log.Printf("TCPSession %p: gosend: stop", h)
            close(h.to_send)
            continue
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
                    h.Events <- Event{"Err", nil}
                    h.conn.Close()
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

    h.wg.Done()
    log.Printf("TCPSession %p: gosend: stopped", h)
}

// Public interface

// Send data
// empty slice = shutdown connection for sending
// Warning: the send queue channel has limited length and may block if called several times in succession.
// Listen for the "Sent" event to throttle
// Must not call Send() after Close() -- the send channel may be closed, and the program will panic.
func (h *TCPSession) Send(data []byte) {
    if data == nil {
        // nil means stop sending
        data = []byte{}
    }
    h.to_send <-data
}

// Close connection
// User must call this to ensure release of all resources associated with TCPSession
func (h *TCPSession) Close() {
    h.conn.Close()   // indirectly stop recv and send goroutines
    h.to_send <- nil // stop send goroutine
    // drain all outstanding events until h.Events closure
    for evt := range h.Events {
        log.Printf("TCPSession %p: drained %s", h, evt.Name)
    }
}
