package goalarmeitbl

import (
	"encoding/hex"
	"fmt"
    "slices"
)

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
func BCD(n int) (byte, error) {
    if n > 99 || n < 0 {
        return 0, fmt.Errorf("valor invalido para BCD: %02x", n)
    }
    return byte(((n / 10) << 4) + (n % 10)), nil
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
