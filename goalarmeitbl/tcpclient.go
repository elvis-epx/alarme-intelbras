package goalarmeitbl

import (
    "net"
    "io"
    "log"
    "slices"
    "sync"
)

// Handle is always called by the same goroutine,
// so there will never be two concurrent calls to Handle()
type TCPClientDelegate interface {
    // If handler may add more than one Event to the events channel, do this in a goroutine
    // otherwise there may be a deadlock
    Handle(*TCPClient, Event) bool
}

type TCPClient struct {
    Events chan Event
    delegate TCPClientDelegate
    RecvBuf []byte

    wg sync.WaitGroup
    to_send chan []byte
    stop_send chan struct{}
    conn *net.TCPConn
}

func NewTCPClient(addr string, delegate TCPClientDelegate) *TCPClient {
    // TODO choose send channel buffer size
    // Event channel capacity at least 2 in case both send() and recv() emit error events and main() has already exited
    // to_send channel capacity at least 1 so main() does block on send()
    // stop_send channel capacity at least 1 so main() does not block on send() when trying to exit

    h := TCPClient{make(chan Event, 2), delegate, nil, sync.WaitGroup{}, make(chan []byte, 1), make(chan struct{}, 1), nil}
    h.wg.Add(1)
    go h.main(addr)
    return &h
}

// Main loop
func (h *TCPClient) main(addr string) {
    defer h.wg.Done()
    go h.connect(addr)

loop:
    for {
        evt := <-h.Events
        log.Print("TCPClient: event ", evt.Name)

        switch evt.Name {
        case "bye":
            break loop
        case "connected":
            defer func() {
                h.stop_send <-struct{}{}
                h.conn.Close()
                // drain events to make room for possible send() and recv() final error events
                for len(h.Events) > 0 {
                    <-h.Events
                }
            }()
            go h.recv()
            go h.send()

            if !h.delegate.Handle(h, Event{"Connected", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }
        case "connerr":
            if !h.delegate.Handle(h, Event{"NotConnected", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }
        case "recv":
            // from recv goroutine
            h.RecvBuf = slices.Concat(h.RecvBuf, evt.Cargo)
            if !h.delegate.Handle(h, Event{"Recv", nil}) {
                log.Fatal("Unhandled event ", evt.Name)
                break loop
            }
        case "send":
            // forward to send goroutine
            h.to_send <-evt.Cargo
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
loop:
    for {
        select {
        case <-h.stop_send:
            break loop
        case data := <-h.to_send:
            if len(data) == 0 {
                // voluntary closure of connection in tx direction
                log.Print("TCPClient: Closing for tx")
                h.conn.CloseWrite()
                close(h.to_send) // make sure program panics if user tries to send anything else
                break loop
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
                    break loop
                }
                log.Print("TCPClient: Sent ", n)
                data = data[n:]
            }
        }
    }
    log.Print("TCPClient: goroutine send stopped")
}

func (h *TCPClient) Send(data []byte) {
    h.Events <-Event{"send", data}
}

func (h *TCPClient) Bye() {
    h.Events <-Event{"bye", nil}
}

func (h *TCPClient) Wait() {
    h.wg.Wait()
}
