package goalarmeitbl

import (
    "net"
    "io"
    "time"
    "log"
)

type tcpclientevent struct {
    name string
    cargo []byte
}

type TCPClient struct {
    Events chan Event

    internal_events chan tcpclientevent
    to_send chan []byte
    conntimeout time.Duration // FIXME allow configuration of connection timeout
    conn *net.TCPConn
}

func NewTCPClient(addr string) *TCPClient {
    h := TCPClient{make(chan Event), make(chan tcpclientevent), make(chan []byte, 2),
                    60 * time.Second, nil}
    go h.main(addr)
    return &h
}

// Main loop
func (h *TCPClient) main(addr string) {
    // client is still interested in this connection?
    active := true

    go h.connect(addr)

    // makes sure events from goroutines are all handled, even if active == false
    connect_complete := false
    send_complete := true
    recv_complete := true

    for active || !connect_complete || !send_complete || !recv_complete {
        evt := <-h.internal_events
        log.Print("TCPClient: gomain: event ", evt.name)

        switch evt.name {

        case "bye":
            if !active {
                break
            }

            active = false
            close(h.Events)

            // stop send goroutine, if running
            close(h.to_send)

            // indirectly stops recv goroutine, if running
            if connect_complete && h.conn != nil {
                h.conn.Close()
            }

        case "connected":
            // event emitted by connect() goroutine
            connect_complete = true

            if !active {
                // "bye" event already happened, close connection early
                h.conn.Close()
                break
            }

            go h.recv()
            recv_complete = false
            go h.send()
            send_complete = false

            h.Events <- Event{"Connected", nil}

        case "connerr":
            // from connect goroutine
            connect_complete = true

            if !active {
                break
            }

            h.Events <- Event{"NotConnected", nil}

        case "sendstop":
            // from send goroutine
            send_complete = true

        case "recvstop":
            // from recv goroutine
            recv_complete = true

        case "recv":
            // from recv goroutine

            if !active {
                break
            }

            h.Events <- Event{"Recv", evt.cargo}

        case "err":
            // notify higher layers

            if !active {
                break
            }

            h.Events <- Event{"Err", nil}

        case "sendeof":
            // notify higher layers

            if !active {
                break
            }

            h.Events <- Event{"SendEof", nil}

        case "recveof":
            // notify higher layers

            if !active {
                break
            }

            h.Events <- Event{"RecvEof", nil}

        default:
            // should not happen
            log.Fatal("Unhandled event ", evt.name)
        }
    }

    log.Print("TCPClient: gomain stopped -------------")
}

// Connection goroutine
func (h *TCPClient) connect(addr string) {
    conn, err := net.DialTimeout("tcp", addr, h.conntimeout)
    if err != nil {
        h.internal_events <-tcpclientevent{"connerr", nil}
        return
    }
    h.conn = conn.(*net.TCPConn)
    h.internal_events <-tcpclientevent{"connected", nil}
}

// Data receiving goroutine
func (h *TCPClient) recv() {
    for {
        data := make([]byte, 1500)
        n, err := h.conn.Read(data)
        if err != nil {
            if err == io.EOF {
                log.Print("TCPClient: gorecv: eof")
                h.internal_events <-tcpclientevent{"recveof", nil}
            } else {
                log.Print("TCPClient: gorecv: err")
                h.internal_events <-tcpclientevent{"err", nil}
            }

            // exit goroutine
            break
        }
        log.Print("TCPClient: gorecv: received ", n)
        h.internal_events <-tcpclientevent{"recv", data[:n]}
    }

    h.internal_events <-tcpclientevent{"recvstop", nil}
    log.Print("TCPClient: gorecv: stopped")
}

// Data sending goroutine
func (h *TCPClient) send() {
    active := true

    for {
        data := <-h.to_send

        if data == nil {
            // exit goroutine
            break
        }

        if !active {
            continue
        }

        if len(data) == 0 {
            log.Print("TCPClient: gosend: shutdown")
            h.conn.CloseWrite()
            active = false
            continue
        }

        for len(data) > 0 {
            log.Print("TCPClient: gosend: sending ", len(data))
            n, err := h.conn.Write(data)

            if err != nil {
                if err == io.EOF {
                    log.Print("TCPClient: gosend: eof")
                    h.internal_events <-tcpclientevent{"sendeof", nil}
                } else {
                    log.Print("TCPClient: gosend: err")
                    h.internal_events <-tcpclientevent{"err", nil}
                }
                active = false
                break
            }

            log.Print("TCPClient: gosend: sent ", n)
            data = data[n:]
        }
    }

    h.internal_events <-tcpclientevent{"sendstop", nil}
    log.Print("TCPClient: gosend: stopped")
}

// Public interface

// Send data
// empty slice = shutdown connection for sending
// Warning: the send queue channel has limited length and may block if called
// several times in succession. Do that in a goroutine (or don't do that at all).
func (h *TCPClient) Send(data []byte) {
    if data == nil {
        // nil slice means closed channel
        data = []byte{}
    }
    // forward to send goroutine
    h.to_send <-data
}

// Close connection and free resources
func (h *TCPClient) Bye() {
    h.internal_events <-tcpclientevent{"bye", nil}
}
