package goalarmeitbl

import (
    "net"
    "time"
    "log"
    "context"
    "sync"
)

type TCPClient struct {
    Events chan Event
    Session *TCPSession
    conntimeout time.Duration
    cancel context.CancelFunc
    state chan string
    closeonce sync.Once
}

type tcpclientevent struct {
    name string
    conn *net.TCPConn
}

// Creates a new TCPClient, that will embed a TCPSession if connection is successful
// User should handle Connected || NotConnected events, and the TCPSession events after Connected

func NewTCPClient(addr string) *TCPClient {
    h := new(TCPClient)
    h.Session = NewTCPSession()
    // using TCPSession channel allows the user to keep listening for the same channel
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
        h.Session.Start(conn.(*net.TCPConn))
        h.Events <- Event{"Connected", nil}
        h.state <- "+"

        log.Printf("TCPClient %p: TCPSession %p in charge -------", h, h.Session)
    }()

    return h
}

// Public interface

// Send data. Forwards to TCPSession.
// May be called only after connection is established. Never blocks.
// empty slice = shutdown connection for sending
// Returns true if send successfully queued, false if queue is full
// Returns false if connection already closed.
// Listen for "Sent" events to manage the queue and avoid failures
func (h *TCPClient) Send(data []byte) bool {
    return h.Session.Send(data)
}

// Close connection, or cancels it if still not established
func (h *TCPClient) Close() {
    h.closeonce.Do(func() {
        // cancel context, if still relevant, to provoke closure of connect goroutine
        h.cancel()

        // wait for connection goroutine to report status, or read its past status
        state := <-h.state

        if state == "-" {
            // not connected. Drain pending events
            for evt := range h.Events {
                log.Printf("TCPClient %p: drained event %s", h, evt.Name)
            }
        } else if state == "+" {
            // already connected; forward
            h.Session.Close() // This method drains pending events by itself
        }
    })
}
