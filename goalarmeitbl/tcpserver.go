package goalarmeitbl

import (
    "net"
    "time"
    "log"
    "errors"
    "sync"
)

type TCPServer struct {
    Events chan Event
    listener net.Listener
    timeouts *TimeoutOwner      // Timeouts associated with this server
    mutex sync.Mutex            // Necessary to close channel among multiple channel writers
}

// Create TCP server
// User must listen "new" Events channel to get TCPSession's and handle the sessions, at least Close() them
// Timeout API: Timeout() to create timeouts owned by this server
// All APIs must not be called after Close()

func NewTCPServer(addr string) (*TCPServer, error) {
    s := new(TCPServer)
    s.Events = make(chan Event, 1)
    s.timeouts = NewTimeoutOwner(s.Events)

    listener, err := net.Listen("tcp", addr)
    if err != nil {
        return nil, err
    }
    s.listener = listener

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
            session := NewTCPSession(s)
            session.Start(conn.(*net.TCPConn))
            s.Events <-Event{"new", session}
        }

        listener.Close()
        s.timeouts.Release()

        // close and nullify Events in tandem
        s.mutex.Lock()
        close(s.Events) // disengage user
        s.Events = nil
        s.mutex.Unlock()

        log.Printf("TCPServer: exited")
    }()

    log.Print("TCPServer: started")
    return s, nil
}

// Should not be called by user. This is a callback for TCPSessions. May be called by any goroutine
func (s *TCPServer) Closed(session *TCPSession) {
    // make sure s.Events remains consistent (open and not nil, or closed and nil)
    s.mutex.Lock()
    defer s.mutex.Unlock()

    if s.Events != nil {
        s.Events <-Event{"closed", session}
        log.Printf("TCPServer %p: closed owned TCPSession %p", s, session)
    } else {
        log.Printf("TCPServer %p: closed disowned TCPSession %p", s, session)
    }
}

// Create new Timeout owned by this server 
// (meaning it is automatically stopped and released when the server is closed)
func (s *TCPServer) Timeout(avgto time.Duration, fudge time.Duration, cbchmsg string) (*Timeout) {
    to := s.timeouts.Timeout(avgto, fudge, cbchmsg)
    log.Printf("TCPServer %p: new owned timeout %p", s, to)
    return to
}

// Stops TCP server. It is guaranteed that no new Events are emitted after this.
// Sessions already accepted by the user are not affected.
func (s *TCPServer) Close() {
    // copy because s.Events will be made nil
    Events := s.Events
    s.listener.Close()
    // drain remaining events until goroutine closes channel
    for evt := range Events {
        log.Printf("TCPSserver %p: drained %s", s, evt.Name)
        if evt.Name == "new" {
            (evt.Cargo.(*TCPSession)).Close()
        }
    }
}
