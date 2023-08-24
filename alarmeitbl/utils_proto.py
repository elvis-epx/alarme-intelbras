#!/usr/bin/env python3

from .myeventloop import Log

class UtilsProtocolo:
    def hexprint(self, buf):
        return ", ".join(["%02x" % n for n in buf])

    # Calcula checksum de frame longo
    # Presume que "dados" contém o byte de comprimento mas não contém o byte de checksum
    def checksum(self, dados):
        checksum = 0
        for n in dados:
            checksum ^= n
        checksum ^= 0xff
        checksum &= 0xff
        return checksum

    # Decodifica número no formato "Contact ID"
    # Retorna -1 se aparenta estar corrompido
    def contact_id_decode(self, dados):
        dados_rev = dados[:]
        dados_rev.reverse()
        numero = 0
        posicao = 1
        for digito in dados_rev:
            if digito == 0x0a: # zero
                pass
            elif digito >= 0x01 and digito <= 0x09:
                numero += posicao * digito
            else:
                Log.warn("valor contact id invalido", self.hexprint(dados))
                return -1
            posicao *= 10
        return numero
    
    # Codifica um número de tamanho fixo no formato Contact-ID
    def contact_id_encode(self, number, length):
        number = abs(number)
        buf = []
        for i in range(0, length):
            digit = number % 10
            number //= 10
            if not digit:
                digit = 0x0a
            buf = [digit] + buf
        return buf
    
    def bcd(self, n):
        if n > 99 or n < 0:
            Log.warn("valor invalido para BCD: %02x" % n)
            return 0
        return ((n // 10) << 4) + (n % 10)
    
    def from_bcd(self, dados):
        n = 0
        dados_rev = dados[:]
        dados_rev.reverse()
        numero = 0
        posicao = 1
        for nibbles in dados_rev:
            numero += (nibbles >> 4) * 10 * posicao
            numero += (nibbles & 0x04) * posicao
            posicao *= 100
        return numero
    
    # Codifica um número de 16 bits em 2 octetos
    def be16(self, n):
        return [ n // 256, n % 256 ]
    
    # Decodifica um buffer de 2 octetos para inteiro de 16 bits
    def parse_be16(self, buf):
        return buf[0] * 256 + buf[1]

    # Codifica um número de tamanho fixo (geralmente uma senha) no formato ISECMobile
    def isecmobile_senha(self, number, length):
        number = abs(number)
        buf = []
        for i in range(0, length):
            digit = number % 10
            buf = [digit + 0x30] + buf
            number //= 10
        return buf

    # Codifica um pacote no formato ISECMobile
    def encode_isecmobile(self, cmd, senha, tamsenha, extra):
        return [ 0x21 ] + self.isecmobile_senha(senha, tamsenha) + cmd + extra + [ 0x21 ]

    # Codifica um pacote no formato ISECNet
    def encode_isecnet(self, cmd, senha, tamsenha, extra):
        pct = self.encode_isecmobile(cmd, senha, tamsenha, extra)
        pct = [ 0xe9 ] + pct
        return [ len(pct) ] + pct + [ self.checksum(pct) ]
