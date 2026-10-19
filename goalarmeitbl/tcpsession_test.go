package goalarmeitbl

import (
	"bufio"
	"bytes"
	"net"
	"testing"
	"time"
)

// Peer sends many packets in a row and the handler replies to each one.
// Replies that don't fit in the send queue are kept and retried on "Sent".
func TestTCPSessionManyReplies(t *testing.T) {
	const port = "127.0.0.1:54323"
	const count = 200

	srv, err := NewTCPServer(port)
	if err != nil {
		t.Fatal("TCP Server creation failed")
	}
	server_stopped := make(chan struct{})

	go func() {
		evt := <-srv.Events
		session := evt.Cargo.(*TCPSession)
		var buf []byte
		var pending [][]byte
		flush := func() {
			for len(pending) > 0 {
				if _, ok := session.Send(pending[0]); !ok {
					break
				}
				pending = pending[1:]
			}
		}
		for evt := range session.Events {
			switch evt.Name {
			case "Recv":
				buf = append(buf, evt.Cargo.([]byte)...)
				for i := bytes.IndexByte(buf, '\n'); i >= 0; i = bytes.IndexByte(buf, '\n') {
					pending = append(pending, append([]byte(nil), buf[:i+1]...))
					buf = buf[i+1:]
				}
				flush()
			case "Sent":
				flush()
			case "SendEof", "RecvEof", "Err":
				session.Close()
			}
		}
		<-srv.Events // "Closed"
		srv.Close()
		close(server_stopped)
	}()

	conn, err := net.Dial("tcp", port)
	if err != nil {
		t.Fatal("Dial failed")
	}
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write(bytes.Repeat([]byte("x\n"), count)); err != nil {
		t.Fatal("Write failed")
	}

	reader := bufio.NewReader(conn)
	for i := 0; i < count; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("reply %d not received: %v", i, err)
		}
	}

	conn.Close()
	<-server_stopped
}

// Send() reports remaining queue capacity, and failure when queue is full
func TestTCPSessionSendCapacity(t *testing.T) {
	// not started, so nothing consumes the send queue
	s := NewTCPSession(nil)
	for _, expected := range []int{1, 0} {
		if n, ok := s.Send([]byte("x")); !ok || n != expected {
			t.Errorf("Send() = %d, %v; expected %d, true", n, ok, expected)
		}
	}
	if n, ok := s.Send([]byte("x")); ok || n != 0 {
		t.Errorf("Send() = %d, %v; expected 0, false", n, ok)
	}
	s.Close()
}
