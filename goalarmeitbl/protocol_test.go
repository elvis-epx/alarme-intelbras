package goalarmeitbl

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestChecksum(t *testing.T) {
	pkt := []byte{0x74, 0x01, 0x35, 0x50, 0x26}
	if Checksum(pkt) != 201 {
		t.Error("failed")
		return
	}
}

func TestHexPrint(t *testing.T) {
	pkt := []byte{0x74, 0x01, 0x35, 0x50, 0x26}
	res := HexPrint(pkt)
	if res != "74 01 35 50 26" {
		t.Error("failed I '" + res + "'")
		return
	}

	pkt2 := []byte{0x74}
	res2 := HexPrint(pkt2)
	if res2 != "74" {
		t.Error("failed II '" + res2 + "'")
		return
	}

	pkt3 := []byte(nil)
	res3 := HexPrint(pkt3)
	if res3 != "" {
		t.Error("failed III '" + res3 + "'")
		return
	}
}

func TestContactIDDecode(t *testing.T) {
	res, err := ContactIDDecode([]byte{0x01, 0x0a, 0x03})
	if res != 103 || err != nil {
		t.Errorf("failed I %d", res)
		return
	}

	res, err = ContactIDDecode([]byte{0x0a, 0x0a, 0x03})
	if res != 3 || err != nil {
		t.Errorf("failed II %d", res)
		return
	}

	res, err = ContactIDDecode([]byte{0x0a, 0x0b, 0x03})
	if err == nil {
		t.Errorf("failed III")
		return
	}
	res, err = ContactIDDecode([]byte{0x0a, 0x00, 0x03})
	if err == nil {
		t.Errorf("failed IV")
		return
	}
}

func TestContactIDEncode(t *testing.T) {
	res := ContactIDEncode(103, 3)
	if !bytes.Equal(res, []byte{0x01, 0x0a, 0x03}) {
		t.Errorf("failed I ")
		return
	}
}

func TestBCD(t *testing.T) {
	res := BCD(89)
	if res != 0x89 {
		t.Errorf("failed I %x", res)
		return
	}

	res = BCD(120)
	if res != 0 {
		t.Errorf("failed II %x", res)
		return
	}
}

func TestFromBCD(t *testing.T) {
	res, err := FromBCD([]byte{0x89})
	if res != 89 || err != nil {
		t.Errorf("failed I %d", res)
		return
	}

	res, err = FromBCD([]byte{0x89, 0x01})
	if res != 8901 || err != nil {
		t.Errorf("failed II %d", res)
		return
	}

	res, err = FromBCD([]byte{0xee})
	if err == nil {
		t.Errorf("failed III %d", res)
		return
	}
}

func TestPacoteIsecNet2Auth(t *testing.T) {
	res := PacoteIsecNet2Auth(123456, 6)
	if !bytes.Equal(res, []byte{0, 0, 143, 255, 0, 10, 240, 240, 2, 1, 2, 3, 4, 5, 6, 16, 144}) {
		t.Errorf("failed I")
		return
	}

	res = PacoteIsecNet2Auth(1034, 4)
	if !bytes.Equal(res, []byte{0, 0, 143, 255, 0, 8, 240, 240, 2, 1, 10, 3, 4, 16, 153}) {
		t.Errorf("failed II")
		return
	}
}

func TestPacoteIsecNet2Completo(t *testing.T) {
	pacote := PacoteIsecNet2Auth(123456, 6)

	res := PacoteIsecNet2Completo(nil)
	if res > 0 {
		t.Errorf("failed nil")
	}

	for i := range len(pacote) - 1 {
		res = PacoteIsecNet2Completo(pacote[:i])
		if res > 0 {
			t.Errorf("failed %d", i)
		}
	}

	res = PacoteIsecNet2Completo(pacote[:len(pacote)])
	if res == 0 {
		t.Errorf("failed full")
	}
}

func TestPacoteIsecNet2Correto(t *testing.T) {
	pacote := PacoteIsecNet2Auth(123456, 6)

	for i := range len(pacote) - 1 {
		if i < 6 {
			// função exige pacotes completos em primeiro lugar
			continue
		}
		res := PacoteIsecNet2Correto(pacote[:i])
		if res {
			t.Errorf("failed %d", i)
		}
	}

	res := PacoteIsecNet2Correto(pacote[:len(pacote)])
	if !res {
		t.Errorf("failed full")
	}
}

func TestPacoteIsecNet2Bye(t *testing.T) {
	res := PacoteIsecNet2Bye()
	if !bytes.Equal(res, []byte{0, 0, 143, 255, 0, 2, 240, 241, 140}) {
		t.Errorf("failed I")
		return
	}
}

func TestPacoteIsecNet2(t *testing.T) {
	res := PacoteIsecNet2(0xbabe, nil)
	if !bytes.Equal(res, []byte{0, 0, 143, 255, 0, 2, 186, 190, 137}) {
		t.Errorf("failed I")
		return
	}

	res = PacoteIsecNet2(0xcafe, []byte{0xba, 0xbe})
	if !bytes.Equal(res, []byte{0, 0, 143, 255, 0, 4, 202, 254, 186, 190, 187}) {
		t.Errorf("failed II")
		return
	}
}

func TestPacoteIsecNet2Parse(t *testing.T) {
	cmd, payload := PacoteIsecNet2Parse(PacoteIsecNet2(0xbabe, nil))
	if cmd != 0xbabe || !bytes.Equal(payload, []byte{}) {
		t.Errorf("failed I %x %d", cmd, len(payload))
		return
	}

	cmd, payload = PacoteIsecNet2Parse(PacoteIsecNet2(0xcafe, []byte{0xba, 0xbe}))
	if cmd != 0xcafe || !bytes.Equal(payload, []byte{0xba, 0xbe}) {
		t.Errorf("failed II %x %d", cmd, len(payload))
		return
	}
}

func TestRIPRespostaDataHora(t *testing.T) {
	// domingo
	data := time.Date(2026, time.October, 11, 23, 4, 59, 0, time.Local)
	esperado := []byte{0x08, 0x80, 0x26, 0x10, 0x11, 0x00, 0x23, 0x04, 0x59}
	esperado = append(esperado, Checksum(esperado))
	if r := RIPRespostaDataHora(data).Encode(); !bytes.Equal(r, esperado) {
		t.Errorf("RIPRespostaDataHora: %s, esperado %s", HexPrint(r), HexPrint(esperado))
	}
}

func TestParseRIPAlarmeTamanhoErrado(t *testing.T) {
	if r := ParseRIPAlarme(PacoteRIP{true, 0xb0, make([]byte, 16)}, false); r.Valido {
		t.Error("evento curto deveria ser inválido")
	}
	if r := ParseRIPAlarme(PacoteRIP{true, 0xb5, make([]byte, 17)}, true); r.Valido {
		t.Error("evento com fotos curto deveria ser inválido")
	}
}

func TestParseRIPAlarmeCampoInvalido(t *testing.T) {
	valido := []byte{0x11, 1, 2, 3, 4, 1, 8, 1, 1, 3, 0x0a, 0x0a, 1, 0x0a, 0x0a, 5, 0x00}
	casos := []struct {
		posicao int
		erro    string
	}{
		{1, "contact_id"},
		{5, "tipo_msg"},
		{8, "código"},
		{11, "partição"},
		{13, "zona"},
	}
	for _, caso := range casos {
		dados := bytes.Clone(valido)
		dados[caso.posicao] = 0x0f
		r := ParseRIPAlarme(PacoteRIP{true, 0xb0, dados}, false)
		if r.Valido || !strings.Contains(r.Erro, caso.erro) {
			t.Errorf("posição %d: erro %q, esperado %q", caso.posicao, r.Erro, caso.erro)
		}
	}
}
