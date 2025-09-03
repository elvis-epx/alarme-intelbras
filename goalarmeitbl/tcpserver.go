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
    timeouts *Parent            // Timeouts associated with this server
    sessions *Parent            // Sessions associated with this server
}

// Create TCP server
// User must listen "New" Events channel to get TCPSession's and handle the sessions, at least Close() them
// Timeout API: Timeout() to create timeouts owned by this server
// All APIs must not be called after Close()

func NewTCPServer(addr string) (*TCPServer, error) {
    s := new(TCPServer)
    // TODO configurable queue length
    s.Events = make(chan Event, 1)
    s.timeouts = NewParent("TCPServer", "Timeout", nil)
    s.sessions = NewParent("TCPServer", "TCPSession", s)

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
            session := NewTCPSession(s.sessions)
            session.Start(conn.(*net.TCPConn))
            s.Events <-Event{"New", session}
        }

        listener.Close()
        s.timeouts.DisownAll()
        s.sessions.DisownAll()
        close(s.Events) // disengage user

        log.Printf("TCPServer: exited")
    }()

    log.Print("TCPServer: started")
    return s, nil
}

// Called back by TCPServer.sessions, before s.timeouts.DisownAll()
func (s *TCPServer) ChildDied(_ string, _ string, child Child) {
    session := child.(*TCPSession)
    s.Events <-Event{"Closed", session}
    // log.Printf("TCPServer %p: closed TCPSession %p", s, session)
}

// Create new Timeout owned by this server 
// (meaning it is automatically stopped and released when the server is closed)
func (s *TCPServer) Timeout(avgto time.Duration, fudge time.Duration, cbchmsg string) (*Timeout) {
    to := NewTimeout(avgto, fudge, s.Events, cbchmsg, s.timeouts)
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
        if evt.Name == "New" {
            (evt.Cargo.(*TCPSession)).Close()
        }
    }
}
