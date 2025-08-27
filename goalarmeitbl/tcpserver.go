package goalarmeitbl

import (
    "net"
    "time"
    "log"
    "errors"
)

type TCPServer struct {
    Events chan Event
    listener net.Listener

    timeouts map[*Timeout]bool  // Timeouts associated with this server
    timeouts_sem chan struct {} // and its semaphore
}

// Create TCP server
// User must listen "new" Events channel to get TCPSession's and handle the sessions, at least Close() them
// Timeout API: Timeout() to create timeouts owned by this server
// All APIs must not be called after Close()

func NewTCPServer(addr string) (*TCPServer, error) {
    s := new(TCPServer)
    s.Events = make(chan Event, 1)

    listener, err := net.Listen("tcp", addr)
    if err != nil {
        return nil, err
    }
    s.listener = listener

    s.timeouts = make(map[*Timeout]bool)
    s.timeouts_sem = make(chan struct{}, 1)
    s.timeouts_sem <-struct{}{}

    go func() {
        for {
            conn, err := listener.Accept()
            if err != nil {
                if errors.Is(err, net.ErrClosed) {
                    break
                }
                log.Printf("TCPServer: accept error: %v", err)
                continue
            }
            log.Print("TCPServer: accept new connection")
            session := NewTCPSession()
            session.Start(conn.(*net.TCPConn))
            s.Events <-Event{"new", session}
        }

        listener.Close()
        s.release_timeouts()

        close(s.Events) // disengage user
        log.Printf("TCPServer: exited")
    }()

    log.Print("TCPServer: started")
    return s, nil
}

// Should not be called by user. This is called by the owned Timeout upon Timeout.Free()
func (s *TCPServer) ReleaseTimeout(to *Timeout) {
    <-s.timeouts_sem
    if _, ok := s.timeouts[to]; ok {
        log.Printf("TCPServer %p: released timeout %p", s, to)
        delete(s.timeouts, to)
    }
    s.timeouts_sem <-struct{}{}
}

// Create new Timeout owned by this server 
// (meaning it is automatically stopped and released when the server is closed)
func (s *TCPServer) Timeout(avgto time.Duration, fudge time.Duration, cbchmsg string) (*Timeout) {
    to := NewTimeout(avgto, fudge, s.Events, cbchmsg, s)

    <-s.timeouts_sem
    s.timeouts[to] = true
    s.timeouts_sem <-struct{}{}
    log.Printf("TCPServer %p: new owned timeout %p", s, to)
    
    return to
}

// Release all owned timeouts upon server closure
func (s *TCPServer) release_timeouts() {
    for {
        var to *Timeout

        // Get some owned timeout
        <-s.timeouts_sem
	    for k := range s.timeouts {
		    to = k
		    break
	    }
        s.timeouts_sem <-struct{}{}

        if to == nil {
            break
        }

        // synchronously calls ReleaseTimeout() and prevents further timeout events
        to.Free()
    }
}

// Stops TCP server. It is guaranteed that no new Events are emitted after this.
// Sessions already accepted by the user are not affected.
func (s *TCPServer) Close() {
    s.listener.Close()
    for evt := range s.Events {
        log.Printf("TCPSserver %p: drained %s", s, evt.Name)
        if evt.Name == "new" {
            (evt.Cargo.(*TCPSession)).Close()
        }
    }
}
