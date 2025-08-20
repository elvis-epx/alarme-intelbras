package goalarmeitbl

import (
    "net"
    "io"
    "time"
    "log"
)

type tcpclientevent struct {
    name string
    data []byte
    conn *net.TCPConn
}

type TCPClient struct {
    Events chan Event

    internal_events chan tcpclientevent
    to_send chan []byte
    conntimeout time.Duration
    conn *net.TCPConn
}

func NewTCPClient(addr string) *TCPClient {
    // FIXME allow configuration of connection timeout
    // FIXME allow configuration of queue depths for high-throughput applications
    send_queue_depth := 2
    // rationale: recverr + senderr + sendstop in case of unexpected close
    minimum_depth := 3

    h := new(TCPClient)
    h.Events = make(chan Event, send_queue_depth + minimum_depth)
    h.internal_events = make(chan tcpclientevent, send_queue_depth + minimum_depth)
    h.to_send = make(chan []byte, send_queue_depth)
    h.conntimeout = 60 * time.Second

    log.Printf("TCPClient %p ==================", h)
    go h.main(addr)
    return h
}

// Main loop
func (h *TCPClient) main(addr string) {
    // client is still interested in this connection?
    active := true

    go h.connect(addr)

    // makes sure events from goroutines are all handled, even if active == false
    connect_finished := false
    send_finished := true
    recv_finished := true

    for active || !connect_finished || !send_finished || !recv_finished {
        evt := <-h.internal_events
        log.Printf("TCPClient %p: gomain: event %s", h, evt.name)

        switch evt.name {

        case "bye":
            if !active {
                break
            }
            active = false
            close(h.Events)  // client disengages
            close(h.to_send) // indirectly stops send goroutine, if running
            if h.conn != nil {
                h.conn.Close() // indirectly stops recv goroutine
            }

        case "connected":
            connect_finished = true

            if !active {
                // "bye" event already happened, close connection early
                evt.conn.Close()
                break
            }

            h.conn = evt.conn
            go h.recv()
            recv_finished = false
            go h.send()
            send_finished = false

            h.Events <- Event{"Connected", nil}

        case "connerr":
            connect_finished = true
            if active { h.Events <- Event{"NotConnected", nil} }

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
            log.Fatal("Unhandled event ", evt.name)
        }
    }

    log.Printf("TCPClient %p: gomain stopped -------------", h)
}

// Connection goroutine
func (h *TCPClient) connect(addr string) {
    conn, err := net.DialTimeout("tcp", addr, h.conntimeout)
    if err != nil {
        h.internal_events <-tcpclientevent{"connerr", nil, nil}
        return
    }
    h.internal_events <-tcpclientevent{"connected", nil, conn.(*net.TCPConn)}
}

// Data receiving goroutine
func (h *TCPClient) recv() {
    for {
        data := make([]byte, 1500)
        n, err := h.conn.Read(data)
        if err != nil {
            if err == io.EOF {
                log.Printf("TCPClient %p: gorecv: eof", h)
                h.internal_events <-tcpclientevent{"recveof", nil, nil}
            } else {
                log.Printf("TCPClient %p: gorecv: err", h)
                h.internal_events <-tcpclientevent{"recverr", nil, nil}
            }
            // exit goroutine
            break
        }
        log.Printf("TCPClient %p: gorecv: received %d", h, n)
        h.internal_events <-tcpclientevent{"recv", data[:n], nil}
    }

    log.Printf("TCPClient %p: gorecv: stopped", h)
}

// Data sending goroutine
func (h *TCPClient) send() {
    is_open := true

    for data := range h.to_send {
        if !is_open {
            continue
        }

        if len(data) == 0 {
            log.Printf("TCPClient %p: gosend: shutdown", h)
            h.conn.CloseWrite()
            is_open = false
            continue
        }

        for len(data) > 0 {
            log.Printf("TCPClient %p: gosend: sending %d", h, len(data))
            n, err := h.conn.Write(data)

            if err != nil {
                if err == io.EOF {
                    log.Printf("TCPClient %p: gosend: eof", h)
                    h.internal_events <-tcpclientevent{"sendeof", nil, nil}
                } else {
                    log.Printf("TCPClient %p: gosend: err", h)
                    h.internal_events <-tcpclientevent{"senderr", nil, nil}
                }
                is_open = false
                break
            }

            log.Printf("TCPClient %p: gosend: sent %d", h, n)
            data = data[n:]
        }

        if is_open {
            h.internal_events <-tcpclientevent{"sent", nil, nil}
        }
    }

    h.internal_events <-tcpclientevent{"sendstop", nil, nil}
    log.Printf("TCPClient %p: gosend: stopped", h)
}

// Public interface

// Send data
// empty slice = shutdown connection for sending
// Warning: the send queue channel has limited length and may block if called several times in succession.
// Listen for the "Sent" event to throttle the calls
// Never call Send() after Bye() -- the send channel will be closed, and the program will panic.
func (h *TCPClient) Send(data []byte) {
    if data == nil {
        // nil slice would mean closed channel
        data = []byte{}
    }
    h.to_send <-data
}

// Close connection
// Also closes channel TCPClient.Events
func (h *TCPClient) Bye() {
    h.internal_events <-tcpclientevent{"bye", nil, nil}
}
