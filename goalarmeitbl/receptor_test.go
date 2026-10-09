package goalarmeitbl

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Monta frame RIP longo: comprimento, tipo, dados, checksum
func frameRIP(tipo byte, dados []byte) []byte {
	pacote := append([]byte{byte(len(dados) + 1), tipo}, dados...)
	return append(pacote, Checksum(pacote))
}

// Dados de evento Contact ID, sem o checksum final
func dadosEvento(qualificador byte, codigo []byte) []byte {
	dados := []byte{0x11, 1, 2, 3, 4, 1, 8, qualificador}
	dados = append(dados, codigo...)
	return append(dados, 0x0a, 1, 0x0a, 0x0a, 5) // partição 1, zona 5
}

// Inicia Receptor IP com ganchos que gravam seus parâmetros em arquivos no diretório dir
func iniciaReceptor(t *testing.T, dir string) string {
	cfg := ReceptorIPConfig{Ganchos: map[string]string{}, Addr: "127.0.0.1", Port: 0}
	for _, tipo := range []string{"central", "ev", "msg", "watchdog"} {
		script := filepath.Join(dir, "gancho_"+tipo)
		saida := filepath.Join(dir, tipo+".txt")
		conteudo := "#!/bin/sh\necho \"$#:$*\" >> " + saida + "\n"
		if err := os.WriteFile(script, []byte(conteudo), 0755); err != nil {
			t.Fatal(err)
		}
		cfg.Ganchos["gancho_"+tipo] = script
	}
	r, err := NewReceptorIP(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return r.tcp.listener.Addr().String()
}

// Aguarda até que o arquivo de saída do gancho tenha n linhas
func linhasGancho(t *testing.T, dir string, tipo string, n int) []string {
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		dados, _ := os.ReadFile(filepath.Join(dir, tipo+".txt"))
		linhas := strings.Split(strings.TrimSpace(string(dados)), "\n")
		if len(dados) > 0 && len(linhas) >= n {
			return linhas
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("gancho %s não recebeu %d invocações", tipo, n)
	return nil
}

func TestReceptorIP(t *testing.T) {
	dir := t.TempDir()
	conn, err := net.Dial("tcp", iniciaReceptor(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	troca := func(nome string, envio []byte) []byte {
		if _, err := conn.Write(envio); err != nil {
			t.Fatal(nome, err)
		}
		buf := make([]byte, 100)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatal(nome, err)
		}
		return buf[:n]
	}
	esperaAck := func(nome string, envio []byte) {
		if resposta := troca(nome, envio); !bytes.Equal(resposta, []byte{0xfe}) {
			t.Errorf("%s: resposta inesperada %s", nome, HexPrint(resposta))
		}
	}

	// identificação: canal 'E', conta 1234 (BCD), MAC parcial
	esperaAck("identificação", frameRIP(0x94, []byte{0x45, 0x12, 0x34, 0xaa, 0xbb, 0xcc}))
	esperaAck("heartbeat", []byte{0xf7})

	// data/hora: resposta longa com tipo 0x80 e checksum válido
	resposta := troca("data/hora", frameRIP(0x80, nil))
	if len(resposta) != 10 || resposta[0] != 0x08 || resposta[1] != 0x80 || Checksum(resposta) != 0 {
		t.Errorf("data/hora: resposta inesperada %s", HexPrint(resposta))
	}

	esperaAck("disparo", frameRIP(0xb0, dadosEvento(1, []byte{1, 3, 0x0a})))

	// restauração dividida em dois segmentos TCP
	frame := frameRIP(0xb0, dadosEvento(3, []byte{1, 3, 0x0a}))
	conn.Write(frame[:5])
	time.Sleep(100 * time.Millisecond)
	esperaAck("restauração", frame[5:])

	esperaAck("evento desconhecido", frameRIP(0xb0, dadosEvento(1, []byte{9, 9, 9})))

	// evento com fotos: checksum "reposicionado" seguido de índice (2 octetos) e número de fotos
	dados := append(dadosEvento(1, []byte{1, 3, 0x0a}), 0x00, 0x00, 0x07, 0x02)
	esperaAck("disparo com fotos", append([]byte{byte(len(dados))}, append([]byte{0xb5}, dados...)...))

	esperaAck("tipo desconhecido", frameRIP(0x99, []byte{0x01}))

	ev := linhasGancho(t, dir, "ev", 4)
	esperado := []string{"4:130 1 5 1", "4:130 1 5 3", "4:999 1 5 1", "4:130 1 5 1"}
	for i := range esperado {
		if ev[i] != esperado[i] {
			t.Errorf("gancho_ev %d: %q, esperado %q", i, ev[i], esperado[i])
		}
	}

	msg := linhasGancho(t, dir, "msg", 4)
	esperado = []string{"Disparo de zona 5", "Restauracao de zona 5", "codigo 999 particao 1 zona 5",
		"Disparo de zona 5 (com fotos, i=7 n=2)"}
	for i := range esperado {
		if !strings.HasPrefix(msg[i], "1:") || !strings.HasSuffix(msg[i], esperado[i]) {
			t.Errorf("gancho_msg %d: %q, esperado ...%q", i, msg[i], esperado[i])
		}
	}
}

func TestReceptorIPEventoInvalido(t *testing.T) {
	dir := t.TempDir()
	conn, err := net.Dial("tcp", iniciaReceptor(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// evento com dígito Contact ID inválido (0x0f) ainda deve ser confirmado
	dados := dadosEvento(1, []byte{1, 3, 0x0f})
	conn.Write(frameRIP(0xb0, dados))
	buf := make([]byte, 10)
	n, err := conn.Read(buf)
	if err != nil || !bytes.Equal(buf[:n], []byte{0xfe}) {
		t.Errorf("resposta inesperada %s %v", HexPrint(buf[:n]), err)
	}

	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "ev.txt")); err == nil {
		t.Error("evento inválido não deveria acionar gancho")
	}
}
