package goalarmeitbl

import (
    "net"
    "io"
    "log"
    "slices"
)

type Delegate func (Event) bool

type TCPClientHandler struct {
    Events chan Event
    delegate Delegate
    RecvBuf []byte

    to_send chan []byte
    conn *net.TCPConn
}

func NewTCPClientHandler(addr string, delegate Delegate) *TCPClientHandler {
    h := TCPClientHandler{make(chan Event), delegate, nil, make(chan []byte, 1), nil}
    go h.main(addr)
    return &h
}

// Main loop
func (h *TCPClientHandler) main(addr string) {
    go h.connect(addr)

loop:
    for {
        evt := <-h.Events
        log.Print("TCP handler: event ", evt.Name)
        switch evt.Name {
        case "connected":
            defer h.conn.Close()
            go h.recv()
            go h.send()
            h.Events <- Event{"Connected", nil}
        case "connerr":
            h.Events <- Event{"NotConnected", nil}
        case "recv":
            // from recv goroutine
            h.RecvBuf = slices.Concat(h.RecvBuf, evt.Cargo)
            h.Events <- Event{"Recv", nil}
        case "send":
            // forward to send goroutine
            h.to_send <- evt.Cargo
        case "err":
            // notify higher layers
            h.Events <- Event{"Err", nil}
        case "eof":
            // notify higher layers
            h.Events <- Event{"Eof", nil}
        case "Bye":
            break loop
        default:
            // try to delegate handling 
            if !h.delegate(evt) {
                log.Fatal("Unhandled event ", evt.Name)
            }
        }
    }
}
func (h *TCPClientHandler) connect(addr string) {
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        h.Events <- Event{"connerr", nil}
        return
    }
    h.conn = conn.(*net.TCPConn)
    h.Events <- Event{"connected", nil}
}

// Data receiving goroutine
func (h *TCPClientHandler) recv() {
    for {
        data := make([]byte, 1500)
        n, err := h.conn.Read(data)
        if err != nil {
            if err == io.EOF {
                h.Events <- Event{"eof", nil}
            } else {
                h.Events <- Event{"err", nil}
            }
            break
        }
        log.Print("Received ", n)
        h.Events <- Event{"recv", data[:n]}
    }
}

// Data sending goroutine
func (h *TCPClientHandler) send() {
    for {
        data := <- h.to_send

        for len(data) > 0 {
            log.Print("Sending ", len(data))
            n, err := h.conn.Write(data)
            if err != nil {
                if err == io.EOF {
                    h.Events <- Event{"eof", nil}
                } else {
                    h.Events <- Event{"err", nil}
                }
                break
            }
            log.Print("Sent ", n)
            data = data[n:]
        }
    }
}
