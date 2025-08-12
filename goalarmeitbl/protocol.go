package goalarmeitbl

import (
	"encoding/hex"
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
        digito := dados[len(dados) - 1 - i]

        if digito == 0x0a { // zero
            // pass
        } else if digito >= 0x01 && digito <= 0x09 {
            numero += posicao * digito
        } else {
            return -1, fmt.ErrorF("valor contact id invalido: %s", HexPrint(dados))
        }
        posicao *= 10

    return numero, nil
}
    
// Codifica um número de tamanho fixo no formato Contact-ID
func ContactIDEncode(number int, length int) ([]byte) {
    dados := make([]byte, length)
    if number < 0 {
        number = -number
    }

    for i in range length {
        digit := number % 10
        number /= 10
        if digit == 0 {
            digit = 0x0a
        }
        dados[length - 1 - i] = digit
    }

    return dados
}

func BCD(n int) (byte, error) {
    if n > 99 || n < 0 {
        return 0, fmt.ErrorF("valor invalido para BCD: %02x", n)
    }
    return ((n / 10) << 4) + (n % 10)
}

func FromBCD(dados []byte) (int, error) {
    numero := 0
    posicao := 1
    
    for i := range dados {
        nibbles := dados[len(dados) - 1 - i]
        numero += (nibbles >> 4) * 10 * posicao
        numero += (nibbles & 0x04) * posicao
        posicao *= 100
    }

    return numero, nil
}


    # Codifica um número de 16 bits em 2 octetos
    def be16(self, n):
        return [ n // 256, n % 256 ]
    
    # Decodifica um buffer de 2 octetos para inteiro de 16 bits
    def parse_be16(self, buf):
        return buf[0] * 256 + buf[1]

    def pacote_isecnet2(self, cmd, payload):
        # ID da central, sempre zero
        dst_id = self.be16(0x0000)
        # ID nosso, pode ser qualquer número, devolvido nos pacotes de retorno
        # Possivelmente uma relíquia de canais seriais onde múltiplos receptores
        # ouvem as mensagens, e dst_id ajudaria a identificar o recipiente
        src_id = self.be16(0x8fff)
        length = self.be16(len(payload) + 2)
        cmd_enc = self.be16(cmd)
        pacote = dst_id + src_id + length + cmd_enc + payload
        pacote = pacote + [ self.checksum(pacote) ]
        return pacote

    def pacote_isecnet2_auth(self, senha, tam_senha):
        # 0x02 software de monitoramento, 0x03 mobile app
        sw_type = [ 0x02 ]
        senha = self.contact_id_encode(senha, tam_senha)
        sw_ver = [ 0x10 ]  # nibble.nibble (0x10 = 1.0)
        payload = sw_type + senha + sw_ver
        return self.pacote_isecnet2(0xf0f0, payload)

    # Retorna o comprimento de um pacote, se houver um pacote completo no buffer
    # Se não, retorna 0
    def pacote_isecnet2_completo(self, data):
        # Um pacote tem tamanho mínimo 9 (src_id, dst_id, len, cmd, checksum)
        if len(data) < 9:
            return 0
        compr = 6 + self.parse_be16(data[4:6]) + 1
        if len(data) < compr:
            return 0
        return compr

    # Consiste um pacote do protocolo ISECNet2
    def pacote_isecnet2_correto(self, pct):
        compr_liquido = self.parse_be16(pct[4:6])
        if compr_liquido < 2:
            # Um pacote deveria ter no minimo um comando
            return False
        # Algoritmo de checksum tem propriedade interessante:
        # checksum de pacote sufixado com checksum resulta em 0
        return self.checksum(pct) == 0x00
    
    # Interpreta um pacote do protocolo ISECNet2
    def pacote_isecnet2_parse(self, pct):
        compr_liquido = self.parse_be16(pct[4:6])
        compr_payload = compr_liquido - 2
        cmd = self.parse_be16(pct[6:8])
        payload = pct[8:8+compr_payload]
        return cmd, payload

    def pacote_isecnet2_bye(self):
        return self.pacote_isecnet2(0xf0f1, [])

