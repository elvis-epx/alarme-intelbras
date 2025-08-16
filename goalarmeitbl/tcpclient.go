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
    // If handler adds more than one Event to the events channel 
    // (e.g. by calling Send() more than once), do this in a goroutine
    // otherwise there may be a deadlock
    Handle(*TCPClient, Event) bool
}

type TCPClient struct {
    Events chan Event
    delegate TCPClientDelegate
    RecvBuf []byte

    wg sync.WaitGroup
    to_send chan []byte
    conn *net.TCPConn
}

func NewTCPClient(addr string, delegate TCPClientDelegate) *TCPClient {
    // TODO choose send channel buffer size
    // Event channel capacity at least 2 in case both send() and recv() emit error events and main() has already exited
    // to_send channel capacity at least 1 so Send() does not block easily

    h := TCPClient{make(chan Event, 2), delegate, nil, sync.WaitGroup{}, make(chan []byte, 1), nil}
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
            // from connect goroutine
            defer func() {
                // indirectly stops recv goroutine
                h.conn.Close()
                // stop send goroutine
                h.to_send <-nil
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
            // from connect goroutine
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
    for {
        data := <-h.to_send

        if data == nil {
            break
        } else if len(data) == 0 {
            // voluntary closure of connection in tx direction
            log.Print("TCPClient: Closing for tx")
            h.conn.CloseWrite()
        } else {
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
                    break
                }
                log.Print("TCPClient: Sent ", n)
                data = data[n:]
            }
        }
    }

    log.Print("TCPClient: goroutine send stopped")
}

// Send data
// empty slice = shutdown connection for sending
func (h *TCPClient) Send(data []byte) {
    if data == nil {
        data = []byte{}
    }
    // forward to send goroutine
    h.to_send <-data
}

// Close connection and free resources
func (h *TCPClient) Bye() {
    h.Events <-Event{"bye", nil}
}

// Wait until connection main loop is stopped
func (h *TCPClient) Wait() {
    h.wg.Wait()
}
