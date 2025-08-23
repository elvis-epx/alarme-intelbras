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
}

type tcpclientevent struct {
    name string
    conn *net.TCPConn
}

func NewTCPClient(addr string) *TCPClient {
    h := new(TCPClient)
    h.Session = NewTCPSession()
    // using TCPSession channel allows the user to keep listening for the same channel
    // regardless of TCPClient or TCPSession being in charge
    h.Events = h.Session.Events
    // FIXME allow to configure connection timeout, or allow to pass a context
    h.conntimeout = 60 * time.Second

    log.Printf("TCPClient %p ==================", h)

    ctx, ctx_cancel := context.WithTimeout(context.Background(), h.conntimeout)
    h.cancel = ctx_cancel

    go func() {
        defer ctx_cancel()

        dialer := &net.Dialer{}
        conn, err := dialer.DialContext(ctx, "tcp", addr)
        if err != nil {
            log.Printf("TCPClient %p: conn fail", h) // including ctx cancellation
            h.Events <- Event{"NotConnected", nil}
            close(h.Events) // user disengages
            return
        }

        log.Printf("TCPClient %p: conn success", h)
        h.Session.Start(conn.(*net.TCPConn))
        h.Events <- Event{"Connected", nil}

        log.Printf("TCPClient %p: TCPSession %p in charge -------", h, h.Session)
    }()

    return h
}

// Public interface

// Cancel connection
// User must still listen for Events and handling them, because Cancel() is asynchronous.
// This method only makes things happen faster than the typical context timeout.
func (h *TCPClient) Cancel() {
    h.cancel()
}

// Send data. Forwards to TCPSession.
// Must be called only after connection is established
// empty slice = shutdown connection for sending
// Warning: the send queue channel has limited length and may block if called several times in succession.
// Listen for the "Sent" event to throttle the calls
// Never call Send() after Bye() -- the send channel will be closed, and the program will panic.
func (h *TCPClient) Send(data []byte) {
    h.Session.Send(data)
}

// Close connection. Forwards to TCPSession.
// Also closes Events channel
// Must be called only after connection is established
func (h *TCPClient) Bye() {
    h.Session.Bye()
}
