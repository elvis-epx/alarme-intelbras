package goalarmeitbl

import (
    "net"
    "io"
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

    wg sync.WaitGroup
    to_send chan []byte
    conn *net.TCPConn
}

func NewTCPClient(addr string, delegate TCPClientDelegate) *TCPClient {
    // Event channel capacity at least 2 in case both send() and recv() emit error events but main() has already exited
    // to_send channel capacity at least 2 so user can call Send(non-empty) plus Send(empty) w/o blocking

    h := TCPClient{make(chan Event, 2), delegate, sync.WaitGroup{}, make(chan []byte, 2), nil}
    h.wg.Add(1)
    go h.main(addr)
    return &h
}

// Main loop
func (h *TCPClient) main(addr string) {
    defer func() {
        // drain outstanding events
        // make room for possible send() and recv() final error events
        for len(h.Events) > 0 {
            <-h.Events
        }
        log.Print("TCPClient: #### goroutine main stopped")
        h.wg.Done()
    }()

    go h.connect(addr)

loop:
    for {
        evt := <-h.Events
        log.Print("TCPClient: event ", evt.Name)

        switch evt.Name {
        case "bye":
            break loop
        case "connected":
            // from connect goroutine
            defer func() {
                // indirectly stops send goroutine
                close(h.to_send)
                // indirectly stops recv goroutine
                h.conn.Close()
            }()

            go h.recv()
            go h.send()

            if !h.delegate.Handle(h, Event{"Connected", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        case "connerr":
            // from connect goroutine
            if !h.delegate.Handle(h, Event{"NotConnected", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        case "recv":
            // from recv goroutine
            if !h.delegate.Handle(h, Event{"Recv", evt.Cargo}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        case "err":
            // notify higher layers
            if !h.delegate.Handle(h, Event{"Err", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        case "sendeof":
            // notify higher layers
            if !h.delegate.Handle(h, Event{"SendEof", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        case "recveof":
            // notify higher layers
            if !h.delegate.Handle(h, Event{"RecvEof", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }

        default:
            if !h.delegate.Handle(h, evt) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }
        }
    }
}

// Connection goroutine
func (h *TCPClient) connect(addr string) {
    conn, err := net.Dial("tcp", addr)
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
                log.Print("TCPClient: recv eof")
                h.Events <-Event{"recveof", nil}
            } else {
                log.Print("TCPClient: recv err")
                h.Events <-Event{"err", nil}
            }
            break
        }
        log.Print("TCPClient: Received ", n)
        h.Events <-Event{"recv", data[:n]}
    }
    log.Print("TCPClient: goroutine recv stopped")
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
            // voluntary closure of connection in tx direction
            log.Print("TCPClient: Closing for tx")
            h.conn.CloseWrite()
            active = false
            continue
        }

        for len(data) > 0 {
            log.Print("TCPClient: Sending ", len(data))
            n, err := h.conn.Write(data)
            if err != nil {
                if err == io.EOF {
                    log.Print("TCPClient: send eof")
                    h.Events <-Event{"sendeof", nil}
                } else {
                    log.Print("TCPClient: send err")
                    h.Events <-Event{"err", nil}
                }
                active = false
                break
            }
            log.Print("TCPClient: Sent ", n)
            data = data[n:]
        }
    }

    log.Print("TCPClient: goroutine send stopped")
}

// Public interface

// Send data
// empty slice = shutdown connection for sending
// Warning: the send queue channel has limited length and may block if called
// several times in a row. Do this in a goroutine (or don't do this at all).
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
