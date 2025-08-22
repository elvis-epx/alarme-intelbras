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
    internal_events chan tcpclientevent
    conntimeout time.Duration
    cancel context.CancelFunc
}

type tcpclientevent struct {
    name string
    conn *net.TCPConn
}

func NewTCPClient(addr string) *TCPClient {
    // FIXME allow to configure connection timeout, or allow to pass a context
    h := new(TCPClient)
    h.Session = NewTCPSession()
    // allows the user to keep listening for the same channel, regardless of
    // TCPClient or TCPSession being in charge
    h.Events = h.Session.Events
    h.internal_events = make(chan tcpclientevent)
    h.conntimeout = 60 * time.Second

    log.Printf("TCPClient %p ==================", h)

    ctx, conn_cancel := context.WithTimeout(context.Background(), h.conntimeout)
    h.cancel = conn_cancel

    go h.connect(addr, ctx)

    go func() {
        evt := <-h.internal_events
        log.Printf("TCPClient %p: gomain: event %s", h, evt.name)
        if evt.name == "connected" {
            h.Session.Start(evt.conn)
            h.Events <- Event{"Connected", nil}
            // TCPSession is in charge of Events from now on
        } else {
            h.Events <- Event{"NotConnected", nil}
            close(h.Events) // user disengages
        }

        conn_cancel()
        log.Printf("TCPClient %p: gomain stopped, TCPSession %p in charge -------", h, h.Session)
    }()

    return h
}

// Connection goroutine
func (h *TCPClient) connect(addr string, ctx context.Context) {
    dialer := &net.Dialer{}
    conn, err := dialer.DialContext(ctx, "tcp", addr)
    if err != nil {
        // ctx timeout/cancel goes through here as well
        h.internal_events <-tcpclientevent{"connerr", nil}
        return
    }
    h.internal_events <-tcpclientevent{"connected", conn.(*net.TCPConn)}
}

// Public interface /////////////////////////////////

// Cancel connection
// User must consider TCPClient to be still "alive", listen for Events and handling them,
// because Cancel() may race with connection establishment. This method exists only to
// make things happen faster than the typical context timeout.
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
// Also closes channel TCPSession.Events
// Must be called only after connection is established
func (h *TCPClient) Bye() {
    h.Session.Bye()
}
