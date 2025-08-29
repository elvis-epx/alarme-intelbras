package goalarmeitbl

import (
    "net"
    "time"
    "log"
    "context"
)

type TCPClient struct {
    Events chan Event
    Session *TCPSession

    conntimeout time.Duration
    cancel context.CancelFunc
    state chan string
}

type tcpclientevent struct {
    name string
    conn *net.TCPConn
}

// Creates a new TCPClient, that will embed a TCPSession if connection is successful
// User should handle Connected || NotConnected events, and the TCPSession events after Connected
func NewTCPClient(addr string) *TCPClient {
    h := new(TCPClient)
    h.Session = NewTCPSession(h)

    // using TCPSession channel allows the user to keep listening to the same channel
    // regardless of TCPClient or TCPSession being in charge
    h.Events = h.Session.Events

    // FIXME allow to configure connection timeout, or allow to pass a context
    h.conntimeout = 60 * time.Second
    h.state = make(chan string, 1)

    log.Printf("TCPClient %p ==================", h)

    ctx, ctx_cancel := context.WithTimeout(context.Background(), h.conntimeout)
    h.cancel = ctx_cancel

    go func() {
        defer ctx_cancel()

        dialer := &net.Dialer{}
        conn, err := dialer.DialContext(ctx, "tcp", addr)
        if err != nil {
            log.Printf("TCPClient %p: conn fail %v", h, err) // including ctx cancellation
            h.Events <- Event{"NotConnected", nil}
            close(h.Events) // user disengages
            h.state <- "-"
            return
        }

        log.Printf("TCPClient %p: conn success", h)
        // start session in two phases to guarantee that
        // a) "Connected" event goes first. Session event starts only after StartB()
        // b) After StartA(), session methods like Send() or Close() can already be called.
        h.Session.StartA(conn.(*net.TCPConn))
        h.Events <- Event{"Connected", nil}
        h.Session.StartB()
        h.state <- "+"

        log.Printf("TCPClient %p: TCPSession %p in charge -------", h, h.Session)
    }()

    return h
}

// not used, implemented just to satisfy the interface
func (h *TCPClient) Closed(_ *TCPSession) {
}

// Public interface

// Send data. Forwards to TCPSession.
// May be called only after connection is established. May block if send queue is full
// Listen for "Sent" events to manage the queue and avoid failures
// Must not be called after Close()
func (h *TCPClient) Send(data []byte) {
    h.Session.Send(data)
}

// Close connection, or cancels it if still not established
func (h *TCPClient) Close() {
    // cancel context, if still relevant, to provoke closure of connect goroutine
    h.cancel()
    // wait for connection goroutine to report status, or read its past status
    <-h.state
    h.Session.Close()
}

func (h *TCPClient) Timeout(avgto time.Duration, fudge time.Duration, cbchmsg string) (*Timeout) {
    return h.Session.Timeout(avgto, fudge, cbchmsg)
}
