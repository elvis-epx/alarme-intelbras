package goalarmeitbl

import (
    "net"
    "log"
    "errors"
)

type TCPServer struct {
    Events chan Event
    listener net.Listener
}

func NewTCPServer(addr string) (*TCPServer, error) {
    s := new(TCPServer)
    s.Events = make(chan Event, 1)

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
            session := NewTCPSession()
            session.Start(conn.(*net.TCPConn))
            s.Events <-Event{"new", session}
        }

        listener.Close()
        close(s.Events) // disengage user
        log.Printf("TCPServer: stopped")
    }()

    log.Print("TCPServer: started")
    return s, nil
}

// User must handle remaining events after calling this
func (s *TCPServer) Stop() {
    s.listener.Close()
}
