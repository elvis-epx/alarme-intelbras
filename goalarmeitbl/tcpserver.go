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

    timeouts *TimeoutOwner      // Timeouts associated with this server

    disowned bool               // For sessions calling Closed() after server already closed
    disowned_sem chan struct {} // and its semaphore
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

    s.disowned = false
    s.disowned_sem = make(chan struct{}, 1)
    s.disowned_sem <-struct{}{}

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
            session.StartA(conn.(*net.TCPConn))
            session.StartB()
            s.Events <-Event{"new", session}
        }

        listener.Close()
        s.timeouts.Release()
        s.disown_sessions()

        // disengage user
        close(s.Events)

        log.Printf("TCPServer: exited")
    }()

    log.Print("TCPServer: started")
    return s, nil
}

// Disown TCPSessions still open because TCPServer is being closed
func (s *TCPServer) disown_sessions() {
    <-s.disowned_sem
    s.disowned = true
    s.disowned_sem <-struct{}{}
}

// Should not be called by user. This is a callback for TCPSessions.
func (s *TCPServer) Closed(session *TCPSession) {
    <-s.disowned_sem
    if !s.disowned {
        s.Events <-Event{"closed", session}
        log.Printf("TCPServer %p: closed owned TCPSession %p", s, session)
    } else {
        log.Printf("TCPServer %p: closed disowned TCPSession %p", s, session)
    }
    s.disowned_sem <-struct{}{}
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
    s.listener.Close()
    for evt := range s.Events {
        log.Printf("TCPSserver %p: drained %s", s, evt.Name)
        if evt.Name == "new" {
            (evt.Cargo.(*TCPSession)).Close()
        }
    }
}
