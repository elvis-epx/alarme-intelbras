package goalarmeitbl

import (
    "net"
    "io"
    "log"
    "errors"
)

type tcpserverevent struct {
    name string
    conn *net.TCPConn
}

type tcpsessionevent struct {
    name string
    data []byte
}

type TCPServer struct {
    Events chan Event
    bye chan struct{}
}

func NewTCPServer(addr string) (*TCPServer, error) {
    s := new(TCPServer)
    s.Events = make(chan Event, 1)
    s.bye = make(chan struct{})

    l, err := net.Listen("tcp", addr)
    if err != nil {
        return nil, err
    }

    go func() {
        <-s.bye
        l.Close()
    }()

    go func() {
        defer l.Close()
        for {
            conn, err := l.Accept()
            if err != nil {
                if errors.Is(err, net.ErrClosed) {
                    // fatal
                    break
                }
                // non-fatal
                log.Printf("TCPServer: accept error: %v", err)
                continue
            }
            log.Print("TCPServer: accept new connection")
            s.Events <-Event{"new", NewTCPSession(conn.(*net.TCPConn))}
        }
        log.Printf("TCPServer: stopped")
    }()

    log.Print("TCPServer: started")
    return s, nil
}

func (s *TCPServer) Bye() {
    close(s.bye)
}

// FIXME merge parts of TCPSession and TCPClient that are equal or similar

type TCPSession struct {
    Events chan Event
    internal_events chan tcpsessionevent
    to_send chan []byte
    conn *net.TCPConn
}

func NewTCPSession(conn *net.TCPConn) *TCPSession {
    // FIXME allow configuration of connection timeout
    // FIXME allow configuration of queue depths for high-throughput applications
    send_queue_depth := 2
    // rationale: recverr + senderr + sendstop in case of unexpected close
    minimum_depth := 3

    h := new(TCPSession)
    h.conn = conn
    h.Events = make(chan Event, send_queue_depth + minimum_depth)
    h.internal_events = make(chan tcpsessionevent, send_queue_depth + minimum_depth)
    h.to_send = make(chan []byte, send_queue_depth)

    go h.recv()
    go h.send()
    go h.main()
    log.Printf("TCPSession %p ==================", h)

    return h
}

// Main loop
func (h *TCPSession) main() {
    // client is still interested in this connection?
    active := true
    // makes sure events from goroutines are all handled, even if active == false
    recv_finished := false
    send_finished := false

    for active || !send_finished || !recv_finished {
        evt := <-h.internal_events
        log.Printf("TCPSession %p: gomain: event %s", h, evt.name)

        switch evt.name {

        case "bye":
            if !active {
                break
            }
            active = false
            close(h.Events)  // user disengages
            close(h.to_send) // indirectly stops send goroutine, if running
            h.conn.Close() // indirectly stops recv goroutine

        case "sent":
            if active { h.Events <- Event{"Sent", len(h.to_send)} }
        case "sendstop":
            send_finished = true
        case "sendeof":
            if active { h.Events <- Event{"SendEof", nil} }
        case "senderr":
            if active { h.Events <- Event{"Err", nil} }

        case "recv":
            if active { h.Events <- Event{"Recv", evt.data} }
        case "recverr":
            recv_finished = true
            if active { h.Events <- Event{"Err", nil} }
        case "recveof":
            recv_finished = true
            if active { h.Events <- Event{"RecvEof", nil} }

        default:
            // should not happen
            log.Fatal("TCPSession: unhandled event ", h, evt.name)
        }
    }

    log.Printf("TCPSession %p: gomain stopped -------------", h)
}

// Data receiving goroutine
func (h *TCPSession) recv() {
    for {
        data := make([]byte, 1500)
        n, err := h.conn.Read(data)
        if err != nil {
            if err == io.EOF {
                log.Printf("TCPSession %p: gorecv: eof", h)
                h.internal_events <-tcpsessionevent{"recveof", nil}
            } else {
                log.Printf("TCPSession %p: gorecv: err or stop", h)
                h.internal_events <-tcpsessionevent{"recverr", nil}
            }
            // exit goroutine
            break
        }
        log.Printf("TCPSession %p: gorecv: received %d", h, n)
        h.internal_events <-tcpsessionevent{"recv", data[:n]}
    }

    log.Printf("TCPSession %p: gorecv: stopped", h)
}

// Data sending goroutine
func (h *TCPSession) send() {
    is_open := true

    for data := range h.to_send {
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
                    h.internal_events <-tcpsessionevent{"sendeof", nil}
                } else {
                    log.Printf("TCPSession %p: gosend: err", h)
                    h.internal_events <-tcpsessionevent{"senderr", nil}
                }
                is_open = false
                break
            }

            log.Printf("TCPSession %p: gosend: sent %d", h, n)
            data = data[n:]
        }

        if is_open {
            h.internal_events <-tcpsessionevent{"sent", nil}
        }
    }

    h.internal_events <-tcpsessionevent{"sendstop", nil}
    log.Printf("TCPSession %p: gosend: stopped", h)
}

// Public interface

// Send data
// empty slice = shutdown connection for sending
// Warning: the send queue channel has limited length and may block if called several times in succession.
// Listen for the "Sent" event to throttle the calls
// Never call Send() after Bye() -- the send channel will be closed, and the program will panic.
func (h *TCPSession) Send(data []byte) {
    if data == nil {
        // nil slice would mean closed channel
        data = []byte{}
    }
    h.to_send <-data
}

// Close connection
// Also closes channel TCPSession.Events
func (h *TCPSession) Bye() {
    h.internal_events <-tcpsessionevent{"bye", nil}
}
