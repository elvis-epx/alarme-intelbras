package goalarmeitbl

import (
    "net"
    "io"
    "time"
    "log"
    "sync"
)

// Handle is always called by the same goroutine,
// so there will never be two concurrent calls to Handle()
type TCPClientDelegate interface {
    // Callee should true only if the event was handled.
    // Unhandled events panic the program to catch bugs
    // Callee should handle at least Connected, NotConnected, Recv, Err, SendEof, RecvEof
    Handle(*TCPClient, Event) bool
}

type TCPClient struct {
    Events chan Event

    delegate TCPClientDelegate
    conntimeout time.Duration // FIXME allow configuration of connection timeout
    wg sync.WaitGroup
    to_send chan []byte
    conn *net.TCPConn
}

func NewTCPClient(addr string, delegate TCPClientDelegate) *TCPClient {
    // Events channel capacity of 2 so at least one send and one receive event will fit w/o blocking
    // to_send channel capacity at least 2 so user can call Send(non-empty) plus Send(empty) w/o blocking
    h := TCPClient{make(chan Event, 2), delegate, 60 * time.Second, sync.WaitGroup{}, make(chan []byte, 2), nil}
    h.wg.Add(1)
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

loop:
    for active || !connect_complete || !send_complete || !recv_complete {
        evt := <-h.Events
        log.Print("TCPClient: gomain: event ", evt.Name)

        switch evt.Name {

        case "bye":
            if !active {
                break
            }

            active = false

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

            if !h.delegate.Handle(h, Event{"Connected", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        case "connerr":
            // from connect goroutine
            connect_complete = true

            if !active {
                break
            }

            if !h.delegate.Handle(h, Event{"NotConnected", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

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

            if !h.delegate.Handle(h, Event{"Recv", evt.Cargo}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        case "err":
            // notify higher layers

            if !active {
                break
            }

            if !h.delegate.Handle(h, Event{"Err", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        case "sendeof":
            // notify higher layers

            if !active {
                break
            }

            if !h.delegate.Handle(h, Event{"SendEof", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        case "recveof":
            // notify higher layers

            if !active {
                break
            }

            if !h.delegate.Handle(h, Event{"RecvEof", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        default:
            // events emitted by other users of the event queue

            if !active {
                break
            }

            if !h.delegate.Handle(h, evt) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }
        }
    }

    log.Print("TCPClient: gomain stopped -------------")
    h.wg.Done()
}

// Connection goroutine
func (h *TCPClient) connect(addr string) {
    conn, err := net.DialTimeout("tcp", addr, h.conntimeout)
    if err != nil {
        h.Events <-Event{"connerr", nil}
        return
    }
    h.conn = conn.(*net.TCPConn)
    h.Events <-Event{"connected", nil}
}

// Data receiving goroutine
func (h *TCPClient) recv() {
    for {
        data := make([]byte, 1500)
        n, err := h.conn.Read(data)
        if err != nil {
            if err == io.EOF {
                log.Print("TCPClient: gorecv: eof")
                h.Events <-Event{"recveof", nil}
            } else {
                log.Print("TCPClient: gorecv: err")
                h.Events <-Event{"err", nil}
            }

            // exit goroutine
            break
        }
        log.Print("TCPClient: gorecv: received ", n)
        h.Events <-Event{"recv", data[:n]}
    }

    h.Events <-Event{"recvstop", nil}
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
                    h.Events <-Event{"sendeof", nil}
                } else {
                    log.Print("TCPClient: gosend: err")
                    h.Events <-Event{"err", nil}
                }
                active = false
                break
            }

            log.Print("TCPClient: gosend: sent ", n)
            data = data[n:]
        }
    }

    h.Events <-Event{"sendstop", nil}
    log.Print("TCPClient: gosend: stopped")
}

// Public interface

// Send data
// empty slice = shutdown connection for sending
// Warning: the send queue channel has limited length and may block if called
// several times in a row. Do that in a goroutine (or don't do that at all).
func (h *TCPClient) Send(data []byte) {
    if data == nil {
        // nil slice means closed channel
        data = []byte{}
    }
    // forward to send goroutine
    h.to_send <-data
}

// Close connection and free resources
// Never blocks the caller
func (h *TCPClient) Bye() {
    go func() {
        h.Events <-Event{"bye", nil}
    }()
}

// Wait until connection main loop is stopped
func (h *TCPClient) Wait() {
    h.wg.Wait()
}
