package goalarmeitbl

import (
	"encoding/hex"
	"fmt"
    "slices"
    "time"
)

var EventosContactID map[int]map[string]string

func init() {
    EventosContactID = map[int]map[string]string{
        100: {"*": "Emergencia medica"},
        110: {"*": "Alarme de incendio"},
        120: {"*": "Panico"},
        121: {"*": "Ativacao/desativacao sob coacao"},
        122: {"*": "Panico silencioso"},
        130: {
            "aber": "Disparo de zona %[1]d",
            "rest": "Restauracao de zona %[1]d",
            },
        133: {"*": "Disparo de zona 24h %[1]d"},
        146: {"*": "Disparo silencioso %[1]d"},
        301: {
            "aber": "Falta de energia AC",
            "rest": "Retorno de energia AC",
            },
        342: {
            "aber": "Falta de energia AC em componente sem fio %[1]d",
            "rest": "Retorno energia AC em componente sem fio %[1]d",
            },
        302: {
            "aber": "Bateria do sistema baixa",
            "rest": "Recuperacao bateria do sistema baixa",
            },
        305: {"*": "Reset do sistema"},
        306: {"*": "Alteracao programacao"},
        311: {
            "aber": "Bateria ausente",
            "rest": "Recuperacao bateria ausente",
            },
        351: {
            "aber": "Corte linha telefonica",
            "rest": "Restauro linha telefonica",
            },
        354: {"*": "Falha ao comunicar evento"},
        147: {
            "aber": "Falha de supervisao %[1]d",
            "rest": "Recuperacao falha de supervisao %[1]d",
            },
        145: {
            "aber": "Tamper em dispositivo expansor %[1]d",
            "rest": "Restauro tamper em dispositivo expansor %[1]d",
            },
        383: {
            "aber": "Tamper em sensor %[1]d",
            "rest": "Restauro tamper em sensor %[1]d",
            },
        384: {
            "aber": "Bateria baixa em componente sem fio %[1]d",
            "rest": "Recuperacao bateria baixa em componente sem fio %[1]d",
            },
        401: {
            "rest": "Ativacao manual P%[2]d",
            "aber": "Desativacao manual P%[2]d",
            },
        403: {
            "rest": "Ativacao automatica P%[2]d",
            "aber": "Desativacao automatica P%[2]d",
            },
        404: {
            "rest": "Ativacao remota P%[2]d",
            "aber": "Desativacao remota P%[2]d",
            },
        407: {
            "rest": "Ativacao remota app P%[2]d",
            "aber": "Desativacao remota app P%[2]d",
            },
        408: {"*": "Ativacao por uma tecla P%[2]d"},
        410: {"*": "Acesso remoto"},
        461: {"*": "Senha incorreta"},
        533: {
            "aber": "Adicao de zona %[1]d",
            "rest": "Remocao de zona %[1]d",
            },
        570: {
            "aber": "Bypass de zona %[1]d",
            "rest": "Cancel bypass de zona %[1]d",
            },
        602: {"*": "Teste periodico"},
        621: {"*": "Reset do buffer de eventos"},
        601: {"*": "Teste manual"},
        616: {"*": "Solicitacao de manutencao"},
        422: {
            "aber": "Acionamento de PGM %[1]d",
            "rest": "Desligamento de PGM %[1]d",
            },
        625: {"*": "Data e hora reiniciados"},
    }
}

// Calcula checksum de frame longo
// Presume que "dados" contém o byte de comprimento mas não contém o byte de checksum
func Checksum(dados []byte) byte {
    chksum := uint8(0)
    for _, n := range dados {
        chksum = chksum ^ n
    }
    chksum = chksum ^ uint8(0xff)
    return chksum
}

// Codifica frame numa forma humanamente legível, em dígitos hexadecimais
func HexPrint(dados []byte) string {
    s := hex.EncodeToString(dados)
    ss := ""
    for i := 0; i < len(s) - 2; i += 2 {
        ss += s[i:i+2]
        ss += " "
    }
    if len(s) > 0 {
        ss += s[len(s)-2:]
    }
    return ss
}

// Decodifica número no formato "Contact ID"
// Retorna -1 se aparenta estar corrompido
func ContactIDDecode(dados []byte) (int, error) {
    numero := 0
    posicao := 1

    for i := range dados {
        digito := int(dados[len(dados) - 1 - i])

        if digito == 0x0a { // zero
            // pass
        } else if digito >= 0x01 && digito <= 0x09 {
            numero += posicao * digito
        } else {
            return -1, fmt.Errorf("valor contact id invalido: %s", HexPrint(dados))
        }
        posicao *= 10
    }

    return numero, nil
}
    
// Codifica um número de tamanho fixo no formato Contact-ID
func ContactIDEncode(number int, length int) ([]byte) {
    dados := make([]byte, length)
    if number < 0 {
        number = -number
    }

    for i := range length {
        digit := number % 10
        number /= 10
        if digit == 0 {
            digit = 0x0a
        }
        dados[length - 1 - i] = byte(digit)
    }

    return dados
}

// Converte um número de até 2 dígitos para BCD
func BCD(n int) byte {
    if n > 99 || n < 0 {
        return 0
    }
    return byte(((n / 10) << 4) + (n % 10))
}

// Converte um número BCD de tamanho arbitrário em inteiro
func FromBCD(dados []byte) (int, error) {
    numero := 0
    posicao := 1
    
    for i := range dados {
        nibbles := int(dados[len(dados) - 1 - i])
        nibble1 := nibbles >> 4
        nibble2 := nibbles & 0x0f
        if nibble1 > 9 || nibble2 > 9 {
            return 0, fmt.Errorf("Codigo BCD invalido: %02x", nibbles)
        }
        numero += nibble1 * 10 * posicao
        numero += nibble2 * posicao
        posicao *= 100
    }

    return numero, nil
}

// Codifica um número de 16 bits em 2 octetos big-endian
func BE16(n int) ([]byte) {
    dados := []byte{byte(n / 256), byte(n % 256)}
    return dados
}
    
// Decodifica um inteiro big endian de exatamente 2 octetos
func ParseBE16(dados []byte) (int) {
    return int(dados[0]) * 256 + int(dados[1])
}

func PacoteIsecNet2(cmd int, payload []byte) []byte {
    // ID da central, sempre zero
    dst_id := BE16(0x0000)
    
    // ID nosso, pode ser qualquer número, devolvido nos pacotes de retorno
    // Possivelmente uma relíquia de canais seriais onde múltiplos receptores
    // ouvem as mensagens, e dst_id ajudaria a identificar o recipiente
    src_id := BE16(0x8fff)
    length := BE16(len(payload) + 2)
    cmd_enc := BE16(cmd)

    pacote := slices.Concat(dst_id, src_id, length, cmd_enc, payload)
    return append(pacote, Checksum(pacote))
}

// Pacote de autenticação ISECNet2
func PacoteIsecNet2Auth(senha int, tamanho_senha int) []byte {
    // 0x02 software de monitoramento, 0x03 mobile app
    sw_type := []byte{ 0x02 }
    enc_senha := ContactIDEncode(senha, tamanho_senha)
    sw_ver := []byte{ 0x10 } // nibble.nibble (0x10 = 1.0)

    payload := slices.Concat(sw_type, enc_senha, sw_ver)
    return PacoteIsecNet2(0xf0f0, payload)
}

// Retorna o comprimento de um pacote, se houver um pacote completo no buffer
// Se não, retorna 0
func PacoteIsecNet2Completo(dados []byte) int {
    // Um pacote tem tamanho mínimo 9 (src_id, dst_id, len, cmd, checksum)
    if len(dados) < 9 {
        return 0
    }
    comprimento := 6 + ParseBE16(dados[4:6]) + 1
    if len(dados) < comprimento {
        return 0
    }
    return comprimento
}

// Consiste um pacote do protocolo ISECNet2 e informa se ele é válido
// Exige que tenha sido pré-testado com PacoteIsecNet2Completo()
func PacoteIsecNet2Correto(dados []byte) bool {
    comprimento_liquido := ParseBE16(dados[4:6])
    if comprimento_liquido < 2 {
        // Um pacote deveria ter no minimo um comando
        return false
    }
        
    // Checksum de pacote já sufixado com checksum é igual a zero   
    return Checksum(dados) == 0
}
 
// Interpreta um pacote do protocolo ISECNet2
// Exige que tenha sido pré-testado com PacoteIsecNet2{Correto, Completo}()
func PacoteIsecNet2Parse(dados []byte) (int, []byte) {
    comprimento_liquido := ParseBE16(dados[4:6])
    comprimento_payload := comprimento_liquido - 2
    cmd := ParseBE16(dados[6:8])
    payload := dados[8:8+comprimento_payload]

    return cmd, payload
}

func PacoteIsecNet2Bye() []byte {
    return PacoteIsecNet2(0xf0f1, nil)
}

type PacoteRIP struct {
    Longo bool
    Tipo int
    Payload []byte
}

func (p PacoteRIP) Encode() []byte {
    if p.Longo {
        pacote := slices.Concat([]byte{byte(len(p.Payload))}, p.Payload)
        pacote = append(pacote, Checksum(pacote))
        return pacote
    } else {
        return p.Payload
    }
}

func RIPRespostaGenerica() PacoteRIP {
     return PacoteRIP{false, 0xfe, []byte{0xfe}}
}

func RIPRespostaDataHora(t time.Time) PacoteRIP {
    year := t.Year()
    month := int(t.Month())
    day := t.Day()
    hour := t.Hour()
    minute := t.Minute()
    second := t.Second()
    // em Go, time.Weekday() retorna 0 para domingo
    // e o protocolo da central adota a mesma convenção
    dow := int(t.Weekday())
    fmt.Printf("RIPRespostaDataHora: %04d-%02d-%02d %02d:%02d:%02d\n", year, month, day, hour, minute, second)

    resposta := []byte{0x80, BCD(year - 2000), BCD(month), BCD(day), BCD(dow), BCD(hour), BCD(minute), BCD(second)}
    return PacoteRIP{true, 0x80, resposta}
}

func ExtrairFrameRIP(buffer []byte) (PacoteRIP, int) {
    if len(buffer) < 1 {
        return PacoteRIP{}, 0
    }
    
    if buffer[0] == 0xf7 {
        return PacoteRIP{false, 0xf7, buffer[0:1]}, 1
    }

    if len(buffer) < 2 {
        return PacoteRIP{}, 0
    }

    esperado := int(buffer[0]) + 2 // comprimento + dados + checksum
    if len(buffer) < esperado {
        return PacoteRIP{}, 0
    }

    rawmsg := buffer[:esperado]

    // checksum de pacote sufixado com checksum resulta em 0
    if Checksum(rawmsg) != 0x00 {
        fmt.Println("ExtrairFrameRIP: checksum errado, rawmsg =", HexPrint(rawmsg))
        return PacoteRIP{true, 0x00, rawmsg}, esperado
    }

    // Mantém checksum no final pois, em algumas mensagens, o último octeto
    // calcula como checksum mas tem outro significado (e.g. 0xb5)
    msg := rawmsg[1:]

    if len(msg) == 0 {
        fmt.Println("ExtrairFrameRIP: mensagem nula")
        return PacoteRIP{true, 0x00, msg}, esperado
    }

    tipo := int(msg[0])
    msg = msg[1:]

    return PacoteRIP{true, tipo, msg}, esperado
}

func ParseRIPIdentificacaoCentral(pacote PacoteRIP) (int, string, bool, string) {
    if len(pacote.Payload) != 7 {
        msg := fmt.Sprintf("ParseRIPIdentificacaoCentral: tamanho inesperado %s", HexPrint(pacote.Payload))
        return 0, "", false, msg
    }

    // canal := msg[0] // 'E' (0x45)=Ethernet, 'G'=GPRS, 'H'=GPRS2
    conta, _ := FromBCD(pacote.Payload[1:3])
    macaddr := HexPrint(pacote.Payload[3:6])

    return conta, macaddr, true, ""
}

type RIPAlarme struct {
    Valido bool
    Erro string
    Canal int
    ContactId int
    Tipo int
    Qualificador int
    Codigo int
    Particao int
    Zona int
    IndiceFotos int
    NrFotos int
    CodigoConhecido bool
    DescricaoHumana string
}      

func ParseRIPAlarme(pacote PacoteRIP, com_foto bool) RIPAlarme {
    res := RIPAlarme{}

    compr := 17
    if com_foto {
        compr = 20
    }

    msg := pacote.Payload

    if len(msg) != compr {
        res.Erro = fmt.Sprintf("ParseRIPAlarme: evento de alarme tamanho inesperado %s", HexPrint(msg))
        return res
    }

    res.Canal = int(msg[0]) // 0x11 Ethernet IP1, 0x12 IP2, 0x21 GPRS IP1, 0x22 IP2

    var err error

    res.ContactId, err = ContactIDDecode(msg[1:5])
    if err != nil {
        res.Erro = "ParseRIPAlarme: contact_id inválido"
        return res
    }

    res.Tipo, err = ContactIDDecode(msg[5:7]) // 18 decimal = Contact ID
    if err != nil {
        res.Erro = "ParseRIPAlarme: tipo_msg inválido"
        return res
    }

    res.Qualificador = int(msg[7])

    res.Codigo, err = ContactIDDecode(msg[8:11])
    if err != nil {
        res.Erro = "ParseRIPAlarme: qualificador inválido"
        return res
    }

    res.Particao, err = ContactIDDecode(msg[11:13])
    if err != nil {
        res.Erro = "ParseRIPAlarme: partição inválida"
        return res
    }

    res.Zona, err = ContactIDDecode(msg[13:16])
    if err != nil {
        res.Erro = "ParseRIPAlarme: zona inválida"
        return res
    }

    res.Valido = true

    if com_foto {
        // checksum := msg[16] // truque do protocolo de reposicionar o checksum
        res.IndiceFotos = int(msg[17]) * 256 + int(msg[18])
        res.NrFotos = int(msg[19])
    }

    evento_contact_id, codigo_conhecido := EventosContactID[res.Codigo]
    qualif_string := ""

    if res.Tipo == 18 && codigo_conhecido {
        if res.Qualificador == 1 {
            qualif_string = "aber"
            _, qualif_conhecido := evento_contact_id[qualif_string]
            if !qualif_conhecido {
                qualif_string = "*"
            }
        } else if res.Qualificador == 3 {
            qualif_string = "rest"
            _, qualif_conhecido := evento_contact_id[qualif_string]
            if !qualif_conhecido {
                qualif_string = "*"
            }
        } else {
            qualif_string = "*"
        }

        padr_descricao, qualif_conhecido := evento_contact_id[qualif_string]
        if qualif_conhecido {
            res.CodigoConhecido = true
            res.DescricaoHumana = fmt.Sprintf(padr_descricao, res.Zona, res.Particao)
            if com_foto {
                fotos := fmt.Sprintf(" (com fotos, i=%d n=%d)", res.IndiceFotos, res.NrFotos)
                res.DescricaoHumana += fotos
            }
        }
    }

    return res
}
