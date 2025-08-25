package goalarmeitbl

import (
    "testing"
    "log"
    "bytes"
    "slices"
)

var (
    SRVPORT = "127.0.0.1:54322"
)

// Server session delegate

type TestServerDelegate struct {
    t *testing.T
    RecvBuf []byte
}

func (d *TestServerDelegate) Handle(c *TCPSession, evt Event) bool {
    log.Printf("test server delegate: event %s", evt.Name)
    switch evt.Name {
        case "Recv":
            received, ok := evt.Cargo.([]byte)
            if !ok {
                log.Fatal("any downcast")
            }
            d.RecvBuf = slices.Concat(d.RecvBuf, received)

            i := bytes.IndexByte(d.RecvBuf, byte('\n'))
            if i == -1 {
                return true
            }

            log.Printf("test server delegate: detected end of packet")
            buffer := make([]byte, i+1, i+1)
            copy(buffer, d.RecvBuf[:i+1])
            d.RecvBuf = d.RecvBuf[i+1:]

            for j := range buffer {
                buffer[j] += 1
            }
            buffer[i] = '\n'
            c.Send(buffer)
            return true

        case "Sent":
            qlen, ok := evt.Cargo.(int)
            if !ok {
                log.Fatal("any downcast queue length")
            }
            log.Print("    packet sent, queue ", qlen)
            return true

        case "SendEof", "RecvEof", "Err":
            c.Close()
            return true
    }
    return false
}

// Client that exercises the server

type TestClientDelegate struct {
    phase int
    t *testing.T
    RecvBuf []byte
}

func (d *TestClientDelegate) Handle(c *TCPClient, evt Event) bool {
    log.Printf("test client delegate: event %s", evt.Name)
    switch evt.Name {
        case "NotConnected":
            d.t.Error("Connection failed")
            return true
        case "Connected":
            c.Send([]byte("abcde\n"))
            return true
        case "Recv":
            received, ok := evt.Cargo.([]byte)
            if !ok {
                log.Fatal("any downcast")
            }
            d.RecvBuf = slices.Concat(d.RecvBuf, received)
            data := string(d.RecvBuf)
            log.Print("    received ", data)
            if d.phase == 0 && data == "bcdef\n" {
                d.RecvBuf = nil
                d.phase = 1
                go func() {
                    c.Send([]byte("01234"))
                    c.Send([]byte("5\n"))
                }()
            } else if d.phase == 1 && data == "123456\n" {
                d.RecvBuf = nil
                d.phase = 2
                c.Send([]byte("xy\n"))
                c.Send(nil)
                c.Send(nil)
                c.Send(nil)
                c.Send(nil)
                c.Send(nil)
                c.Send(nil)
                c.Send(nil)
                c.Send(nil)
                c.Send(nil)
                c.Send(nil)
                c.Send(nil)
                c.Send(nil)
            } else if d.phase == 2 && data == "yz\n" {
                d.RecvBuf = nil
                c.Close()
            }
            return true
        case "Sent":
            qlen, ok := evt.Cargo.(int)
            if !ok {
                log.Fatal("any downcast queue length")
            }
            log.Print("    packet sent, queue ", qlen)
            return true
        case "SendEof", "RecvEof", "Err":
            c.Close()
            return true
    }
    return false
}

// test body

func TestTCPServer(t *testing.T) {
    srv, err := NewTCPServer(SRVPORT)
    if err != nil {
        t.Error("TCP Server creation failed")
        return
    }
    go func() {
        // Handles only one session and quits
        evt := <-srv.Events
        session := evt.Cargo.(*TCPSession)

        sd := new(TestServerDelegate)
        sd.t = t
 
        for session_evt := range session.Events {
            if !sd.Handle(session, session_evt) {
                t.Error("Unhandled session event ", session_evt)
                break
            }
        }

        srv.Stop()
    }()

    // auxiliary client that exercises the server

    d := new(TestClientDelegate)
    d.phase = 0
    d.t = t

    c := NewTCPClient(SRVPORT)
    for evt := range c.Events {
        if !d.Handle(c, evt) {
            t.Error("Unhandled client event")
            break
        }
    }
}
