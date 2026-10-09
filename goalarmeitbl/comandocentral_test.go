package goalarmeitbl

import (
    "net"
    "testing"
    "time"
)

// Central falsa: responde cada pacote recebido com a próxima resposta da lista
func centralFalsa(t *testing.T, respostas ...[]byte) string {
    listener, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatal(err)
    }
    go func() {
        defer listener.Close()
        conn, err := listener.Accept()
        if err != nil {
            return
        }
        defer conn.Close()
        buf := make([]byte, 100)
        for _, resposta := range respostas {
            if _, err := conn.Read(buf); err != nil {
                return
            }
            conn.Write(resposta)
        }
        conn.SetReadDeadline(time.Now().Add(5 * time.Second))
        for {
            if _, err := conn.Read(buf); err != nil {
                return
            }
        }
    }()
    return listener.Addr().String()
}

func resultadoComando(t *testing.T, respostas ...[]byte) int {
    sub, _ := NewSolicitarStatus(0)
    c := NewComandoCentral(sub, centralFalsa(t, respostas...), 1234, 4)
    select {
    case res := <-c.resultado:
        return res
    case <-time.After(5 * time.Second):
        t.Fatal("comando não concluiu")
    }
    return -1
}

func TestComandoStatusCurto(t *testing.T) {
    auth_ok := PacoteIsecNet2(0xf0f0, []byte{0x00})
    status_curto := PacoteIsecNet2(0x0b4a, []byte{0x01, 0x02, 0x03})
    if resultadoComando(t, auth_ok, status_curto) != 1 {
        t.Error("status curto deveria resultar em falha")
    }
}

func TestComandoChecksumErrado(t *testing.T) {
    pacote := PacoteIsecNet2(0xf0f0, []byte{0x00})
    pacote[len(pacote) - 1] ^= 0xff
    if resultadoComando(t, pacote) != 1 {
        t.Error("checksum errado deveria resultar em falha")
    }
}
