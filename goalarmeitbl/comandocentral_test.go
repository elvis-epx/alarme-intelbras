package goalarmeitbl

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// Central falsa: responde cada pacote recebido com a próxima resposta da lista
// Os pacotes recebidos são devolvidos pelo canal retornado, quando a conexão termina
func centralFalsa(t *testing.T, respostas ...[]byte) (string, chan [][]byte) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	recebidos := make(chan [][]byte, 1)
	go func() {
		defer listener.Close()
		var pacotes [][]byte
		defer func() { recebidos <- pacotes }()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		for i := 0; ; i++ {
			buf := make([]byte, 100)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			pacotes = append(pacotes, buf[:n])
			if i < len(respostas) {
				conn.Write(respostas[i])
			}
		}
	}()
	return listener.Addr().String(), recebidos
}

func executaComando(t *testing.T, sub ComandoCentralSub, respostas ...[]byte) (int, [][]byte) {
	addr, recebidos := centralFalsa(t, respostas...)
	c := NewComandoCentral(sub, addr, 1234, 4)
	select {
	case res := <-c.resultado:
		select {
		case pacotes := <-recebidos:
			return res, pacotes
		case <-time.After(5 * time.Second):
			t.Fatal("central falsa não encerrou")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("comando não concluiu")
	}
	return -1, nil
}

func resultadoComando(t *testing.T, respostas ...[]byte) int {
	sub, _ := NewSolicitarStatus(0)
	res, _ := executaComando(t, sub, respostas...)
	return res
}

var authOk = PacoteIsecNet2(0xf0f0, []byte{0x00})
var ack = PacoteIsecNet2(0xf0fe, nil)

func TestComandos(t *testing.T) {
	status := make([]byte, 64)
	status[0] = 0x01 // AMT-8000

	casos := []struct {
		comando  string
		extra    int
		cmd      int    // comando esperado após autenticação
		payload  []byte // payload esperado
		resposta []byte
	}{
		{"nulo", 0, -1, nil, nil},
		{"status", 0, 0x0b4a, []byte{}, PacoteIsecNet2(0x0b4a, status)},
		{"ativar", 2, 0x401e, []byte{0x02, 0x01}, PacoteIsecNet2(0x401e, nil)},
		{"ativar", 0, 0x401e, []byte{0xff, 0x01}, PacoteIsecNet2(0x401e, nil)},
		{"desativar", 0, 0x401e, []byte{0xff, 0x00}, PacoteIsecNet2(0x401e, nil)},
		{"desligarsirene", 0, 0x4019, []byte{0xff}, ack},
		{"limpardisparo", 0, 0x4013, []byte{}, ack},
		{"bypass", 5, 0x401f, []byte{0x04, 0x01}, ack},
		{"cancelbypass", 5, 0x401f, []byte{0x04, 0x00}, ack},
	}

	for _, caso := range casos {
		t.Run(caso.comando, func(t *testing.T) {
			sub, erro := Subcomandos[caso.comando].Construtor(caso.extra)
			if erro != "" {
				t.Fatal(erro)
			}
			respostas := [][]byte{authOk}
			if caso.resposta != nil {
				respostas = append(respostas, caso.resposta)
			}
			res, pacotes := executaComando(t, sub, respostas...)
			if res != 0 {
				t.Errorf("resultado %d, esperado sucesso", res)
			}

			// autenticação, [comando], despedida
			if !bytes.Equal(pacotes[0], PacoteIsecNet2Auth(1234, 4)) {
				t.Errorf("autenticação inesperada %s", HexPrint(pacotes[0]))
			}
			if caso.cmd >= 0 {
				cmd, payload := PacoteIsecNet2Parse(pacotes[1])
				if cmd != caso.cmd || !bytes.Equal(payload, caso.payload) {
					t.Errorf("comando inesperado %s", HexPrint(pacotes[1]))
				}
			}
			if !bytes.Equal(pacotes[len(pacotes)-1], PacoteIsecNet2Bye()) {
				t.Errorf("despedida inesperada %s", HexPrint(pacotes[len(pacotes)-1]))
			}
		})
	}
}

func TestComandosFalha(t *testing.T) {
	casos := []struct {
		nome      string
		respostas [][]byte
	}{
		{"senha incorreta", [][]byte{PacoteIsecNet2(0xf0f0, []byte{0x01})}},
		{"nak", [][]byte{PacoteIsecNet2(0xf0fd, []byte{0x01})}},
		{"central ocupada", [][]byte{PacoteIsecNet2(0xf0f7, nil)}},
		{"resposta inesperada", [][]byte{authOk, ack}},
		{"status curto", [][]byte{authOk, PacoteIsecNet2(0x0b4a, []byte{0x01, 0x02, 0x03})}},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if resultadoComando(t, caso.respostas...) != 1 {
				t.Error("deveria resultar em falha")
			}
		})
	}
}

func TestComandoChecksumErrado(t *testing.T) {
	pacote := PacoteIsecNet2(0xf0f0, []byte{0x00})
	pacote[len(pacote)-1] ^= 0xff
	if resultadoComando(t, pacote) != 1 {
		t.Error("checksum errado deveria resultar em falha")
	}
}

func TestComandoZonaInvalida(t *testing.T) {
	for _, comando := range []string{"bypass", "cancelbypass"} {
		for _, zona := range []int{0, 255} {
			if _, erro := Subcomandos[comando].Construtor(zona); erro == "" {
				t.Errorf("%s zona %d deveria ser rejeitado", comando, zona)
			}
		}
	}
}
