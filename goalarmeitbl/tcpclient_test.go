package goalarmeitbl

import (
    "testing"
    "log"
    "bytes"
    "slices"
    "time"
    "net"
    "io"
)

var (
    PORT = "127.0.0.1:54321"
)

// test TCP server
func tcpserver() {
    l, err := net.Listen("tcp4", PORT)
    if err != nil {
        log.Fatal(err)
    }

    go func() {
        defer l.Close()

        c, err := l.Accept()
        if err != nil {
            log.Fatal(err)
        }
        go tcpserver_handle(c)
    }()
}

// test TCP server connection handler
// Receive packet delimited by \n, add 0x01 to each octet and send it back
func tcpserver_handle(c net.Conn) {
    packet := make([]byte, 0)
    tmp := make([]byte, 4096)
    defer c.Close()

    active := true

    for active {
        n, err := c.Read(tmp)
        if err != nil {
            if err != io.EOF {
                log.Fatal("read error: ", err)
            }
            active = false
            log.Printf("tcpserver: read EOF")
        } else {
            log.Printf("tcpserver: received %d bytes", n)
            packet = append(packet, tmp[:n]...)
        }

        i := bytes.IndexByte(packet, byte('\n'))
        if i != -1 {
            log.Printf("tcpserver: detected end of packet")
            buffer := make([]byte, i+1, i+1)
            copy(buffer, packet[:i+1])
            packet = packet[i+1:]

            for j := range buffer {
                buffer[j] += 1
            }
            buffer[i] = '\n'
            _, err := c.Write(buffer)
            if err != nil {
                if err != io.EOF {
                    log.Fatal("write error: ", err)
                }
                break
            }
        }
    }
}

type TestDelegate struct {
    phase int
    t *testing.T
    RecvBuf []byte
}

func (d *TestDelegate) Handle(c *TCPClient, evt Event) bool {
    log.Printf("test delegate: event %s", evt.Name)
    switch evt.Name {
        case "NotConnected":
            c.Bye()
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
                go func() {
                    c.Send([]byte("xy\n"))
                    c.Send(nil)
                }()
            } else if d.phase == 2 && data == "yz\n" {
                d.RecvBuf = nil
                c.Bye()
            }
            return true
        case "SendEof":
            c.Bye()
            return true
        case "RecvEof":
            c.Bye()
            return true
        case "Err":
            c.Bye()
            return true
    }
    return false
}

func TestTCPClient(t *testing.T) {
    tcpserver()

    d := new(TestDelegate)
    d.phase = 0
    d.t = t

    c := NewTCPClient(PORT)
    for evt := range c.Events {
        if !d.Handle(c, evt) {
            log.Fatal("Unhandled event ", evt.Name)
        }
    }
}

type TestDelegate2 struct {
    t *testing.T
}

func (d *TestDelegate2) Handle(c *TCPClient, evt Event) bool {
    log.Printf("test delegate 2: event %s", evt.Name)
    switch evt.Name {
        case "NotConnected":
            c.Bye()
            return true
    }
    return false
}

func TestTCPClient2(t *testing.T) {
    d := new(TestDelegate2)
    d.t = t

    c := NewTCPClient(PORT)

    for evt := range c.Events {
        if !d.Handle(c, evt) {
            log.Fatal("Unhandled event ", evt.Name)
        }
    }
}

type TestDelegate3 struct {
    t *testing.T
    client *TCPClient
}

func (d *TestDelegate3) Handle(c *TCPClient, evt Event) bool {
    log.Printf("test delegate 3: event %s", evt.Name)
    switch evt.Name {
        case "to":
            log.Print("simulating connection failure")
            d.client.conn.Close()
            return true
        case "Connected":
            return true
        case "Err":
            c.Bye()
            return true
    }
    return false
}

func TestTCPClient3(t *testing.T) {
    tcpserver()
    d := new(TestDelegate3)
    d.t = t

    c := NewTCPClient(PORT)
    d.client = c

    NewTimeout(2 * time.Second, 0, c.Events, "to")

    for evt := range c.Events {
        if !d.Handle(c, evt) {
            log.Fatal("Unhandled event ", evt.Name)
        }
    }
}
