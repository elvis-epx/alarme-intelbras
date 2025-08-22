package goalarmeitbl

import (
    "net"
    "log"
    "errors"
)

type tcpserverevent struct {
    name string
    conn *net.TCPConn
}

type TCPServer struct {
    Events chan Event
    bye chan struct{}
}

func NewTCPServer(addr string) (*TCPServer, error) {
    s := new(TCPServer)
    s.Events = make(chan Event, 1)
    s.bye = make(chan struct{})

    l, err := net.Listen("tcp", addr)
    if err != nil {
        return nil, err
    }

    // Closes socket when user calls Bye()
    go func() {
        <-s.bye
        l.Close()
    }()

    go func() {
        for {
            conn, err := l.Accept()
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

        l.Close()
        close(s.Events) // user disengages
        log.Printf("TCPServer: stopped")
    }()

    log.Print("TCPServer: started")
    return s, nil
}

// User must handle remaining events after calling this
func (s *TCPServer) Bye() {
    close(s.bye)
}
