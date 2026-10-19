package goalarmeitbl

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"
)

type TCPServer struct {
	Events   chan Event
	listener net.Listener
	timeouts *Parent // Timeouts associated with this server
	sessions *Parent // Sessions associated with this server

	closed_mutex sync.Mutex // concurrency between ChildDied() and closure of Events
	closed       bool
}

// Create TCP server
// User must listen "New" Events channel to get TCPSession's and handle the sessions, at least Close() them
// Timeout API: Timeout() to create timeouts owned by this server
// All APIs must not be called after Close()

func NewTCPServer(addr string) (*TCPServer, error) {
	s := new(TCPServer)
	// TODO configurable queue length. May be bufferless
	s.Events = make(chan Event)
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
			s.Events <- Event{"New", session}
		}

		listener.Close()
		s.timeouts.DisownAll()
		s.sessions.DisownAll()
		s.closed_mutex.Lock()
		s.closed = true
		close(s.Events) // disengage user
		s.closed_mutex.Unlock()

		log.Printf("TCPServer: exited")
	}()

	log.Print("TCPServer: started")
	return s, nil
}

// Called back by TCPServer.sessions, possibly concurrently with server closure
func (s *TCPServer) ChildDied(_ string, _ string, child Child) {
	s.closed_mutex.Lock()
	defer s.closed_mutex.Unlock()
	if s.closed {
		return
	}
	session := child.(*TCPSession)
	s.Events <- Event{"Closed", session}
	// log.Printf("TCPServer %p: closed TCPSession %p", s, session)
}

// Create new Timeout owned by this server
// (meaning it is automatically stopped and released when the server is closed)
func (s *TCPServer) Timeout(avgto time.Duration, fudge time.Duration, cbchmsg string) *Timeout {
	to := NewTimeout(avgto, fudge, s.Events, cbchmsg, s.timeouts)
	log.Printf("TCPServer %p: new owned timeout %p", s, to)
	return to
}

// Stops TCP server. It is guaranteed that no new Events are emitted after this.
// Sessions already accepted by the user are not affected.
func (s *TCPServer) Close() {
	s.listener.Close()
	// drain remaining events until goroutine closes channel
	for evt := range s.Events {
		log.Printf("TCPSserver %p: drained %s", s, evt.Name)
		if evt.Name == "New" {
			(evt.Cargo.(*TCPSession)).Close()
		}
	}
}
