#!/usr/bin/env python3

import time
from .myeventloop.tcpclient import *
from .utils_proto import *

# Envio de comandos diretamente do cliente para a central de alarme
# usando o novo protocolo ISECNet2 (o mesmo do download de fotos)

class ComandarCentral(TCPClientHandler, UtilsProtocolo):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__((ip_addr, cport))
        self.log_debug("Inicio")
        self.observer = observer
        self.conn_timeout = self.timeout("conn_timeout", 15, self.conn_timeout)
        self.senha = senha
        self.tam_senha = tam_senha
        self.extra = extra
        self.status = 1
        self.tratador = None

    def destroyed_callback(self):
        if self.observer:
            self.observer.resultado(self.status)

    def conn_timeout(self, task):
        self.log_info("Timeout")
        if self.status != 0:
            # se status == 0, provavelmente já completou a tarefa e o timeout é na despedida
            self.status = 1
        self.destroy()

    def connection_callback(self, ok):
        self.conn_timeout.cancel()
        if not ok:
            self.status = 1
            self.log_info("Conexao falhou")
            # destroy() executado pelo chamador
            return
        self.autenticacao()

    def autenticacao(self):
        self.log_debug("Autenticacao")
        pct = self.pacote_isecnet2_auth(self.senha, self.tam_senha)
        self.log_debug("Send", self.hexprint(pct))
        self.send(pct)
        self.tratador = self.resposta_autenticacao
        self.conn_timeout.restart()

    def recv_callback(self, latest):
        self.log_debug("Recv", self.hexprint(latest))

        compr = self.pacote_isecnet2_completo(self.recv_buf)
        if not compr:
            self.log_debug("Pacote incompleto")
            return

        pct, self.recv_buf = self.recv_buf[:compr], self.recv_buf[compr:]

        if not self.pacote_isecnet2_correto(pct):
            self.log_info("Pacote incorreto, desistindo")
            self.destroy()
            return

        cmd, payload = self.pacote_isecnet2_parse(pct)
        self.log_debug("Resposta %04x" % cmd)

        if not self.tratador:
            self.log_info("Sem tratador")
            self.destroy()
            return

        self.conn_timeout.cancel()
        self.tratador(cmd, payload)

    def resposta_autenticacao(self, cmd, payload):
        if cmd == 0xf0fd:
            self.nak(payload)
            return

        if cmd != 0xf0f0:
            self.log_info("Autenticacao: resp inesperada %04x" % cmd)
            self.destroy()
            return

        if len(payload) != 1:
            self.log_info("Autenticacao: resposta invalida")
            self.destroy()
            return

        resposta = payload[0]
        # Possíveis respostas:
        # 01 = senha incorreta
        # 02 = versão software incorreta
        # 03 = painel chamará de volta (?)
        # 04 = aguardando permissão de usuário (?)
        if resposta > 0:
            self.log_info("Autenticacao: falhou motivo %d" % resposta)
            self.destroy()
            return

        self.log_debug("Autenticacao ok")
        self.envia_comando_in()

    def envia_comando(self, cmd, payload, tratador_in):
        pct = self.pacote_isecnet2(cmd, payload)
        self.log_debug("Send", self.hexprint(pct))
        self.send(pct)

        self.cmd = cmd
        self.tratador = self.resposta_comando
        self.tratador_in = tratador_in

        self.conn_timeout.restart()

    def resposta_comando(self, cmd, payload):
        if cmd == 0xf0fd:
            self.nak(payload)
            return

        if cmd == 0xf0f7:
            self.log_info("Erro central ocupada")
            self.destroy()
            return

        if cmd != self.cmd and cmd != 0xf0fe:
            self.log_info("Resposta inesperada %04x" % cmd)
            self.destroy()
            return

        self.tratador_in(payload)

    def despedida(self):
        self.log_debug("Despedindo")
        pct = self.pacote_isecnet2_bye()
        self.log_debug("Send", self.hexprint(pct))
        self.send(pct)

        self.tratador = None
        # Reportar sucesso ao observador
        self.status = 0
        self.conn_timeout.restart()
        # Resposta esperada: central fechar conexão

    def nak(self, payload):
        if len(payload) != 1:
            self.log_info("NAK invalido")
        else:
            motivo = payload[0]
            self.log_info("NAK motivo %02x" % motivo)
            self.status = 1
        self.destroy()


class AtivarDesativarCentral(ComandarCentral):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra, subcmd):
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra)
        self.particao = extra[0]
        self.subcmd = subcmd

    def envia_comando_in(self):
        # byte 1: particao (0x01 = 1, 0xff = todas ou sem particao)
        # byte 2: 0x00 desarmar, 0x01 armar, 0x02 stay
        if self.particao is None:
            payload = [ 0xff, self.subcmd ]
        else:
            payload = [ self.particao, self.subcmd ]
        self.envia_comando(0x401e, payload, self.resposta_comando_in)

    def resposta_comando_in(self, payload):
        self.despedida()

class DesativarCentral(AtivarDesativarCentral):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra, 0x00)

class AtivarCentral(AtivarDesativarCentral):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra, 0x01)


class DesligarSirene(ComandarCentral):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra)
        self.particao = extra[0]

    def envia_comando_in(self):
        if self.particao is None:
            payload = [ 0xff ]
        else:
            payload = [ self.particao]
        self.envia_comando(0x4019, payload, self.resposta_comando_in)

    def resposta_comando_in(self, payload):
        self.despedida()


class LimparDisparo(ComandarCentral):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra)

    def envia_comando_in(self):
        self.envia_comando(0x4013, [], self.resposta_comando_in)

    def resposta_comando_in(self, payload):
        self.despedida()


class CancelarZona(ComandarCentral):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra)
        self.zona = extra[0]

    def envia_comando_in(self):
        # TODO Suportar todas as zonas (enviar 0xff)
        if not self.zona or self.zona < 1 or self.zona > 254:
            raise Exception("Zona precisa ser especificada.")
        payload = [ self.zona - 1, 0x01 ]
        self.envia_comando(0x401f, payload, self.resposta_comando_in)

    def resposta_comando_in(self, payload):
        self.despedida()

class ReativarZona(ComandarCentral):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra)
        self.zona = extra[0]

    def envia_comando_in(self):
        # TODO Suportar todas as zonas (enviar 0xff)
        if not self.zona or self.zona < 1 or self.zona > 254:
            raise Exception("Zona precisa ser especificada.")
        payload = [ self.zona - 1, 0x00 ]
        self.envia_comando(0x401f, payload, self.resposta_comando_in)

    def resposta_comando_in(self, payload):
        self.despedida()


class SolicitarStatus(ComandarCentral):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra)

    def envia_comando_in(self):
        self.envia_comando(0x0b4a, [], self.resposta_comando_in)

    def resposta_comando_in(self, payload):
        # Documentação é base 1
        payload = [0] + payload
        print()
        print("*******************************************")
        if payload[1] == 0x01:
            print("Central AMT-8000")
        else:
            print("Central de tipo desconhecido")
        print("Versão de firmware %d.%d.%d" % tuple(payload[2:5]))
        print("Status geral: ")
        armado = {0x00: "Desarmado", 0x01: "Partição(ões) armada(s)", 0x03: "Todas partições armadas"}
        print("\t" + armado[((payload[21] >> 5) & 0x03)])
        print("\tZonas em alarme:", (payload[21] & 0x8) and "Sim" or "Não")
        print("\tZonas canceladas:", (payload[21] & 0x10) and "Sim" or "Não")
        print("\tTodas zonas fechadas:", (payload[21] & 0x4) and "Sim" or "Não")
        print("\tSirene:", (payload[21] & 0x2) and "Sim" or "Não")
        print("\tProblemas:", (payload[21] & 0x1) and "Sim" or "Não")
        for particao in range(0, 17):
            habilitado = payload[22 + particao] & 0x80
            if not habilitado:
                continue
            print("Partição %02d:" % particao)
            print("\tStay:", (payload[22 + particao] & 0x40) and "Sim" or "Não")
            print("\tDelay de saída:", (payload[22 + particao] & 0x20) and "Sim" or "Não")
            print("\tPronto para armar:", (payload[22 + particao] & 0x10) and "Sim" or "Não")
            print("\tAlame ocorreu:", (payload[22 + particao] & 0x08) and "Sim" or "Não")
            print("\tEm alarme:", (payload[22 + particao] & 0x04) and "Sim" or "Não")
            print("\tArmado modo stay:", (payload[22 + particao] & 0x02) and "Sim" or "Não")
            print("\tArmado:", (payload[22 + particao] & 0x01) and "Sim" or "Não")
        print("Zonas abertas:", self.bits_para_numeros(payload[39:47]))
        print("Zonas em alarme:", self.bits_para_numeros(payload[47:55]))
        # print("Zonas ativas:", self.bits_para_numeros(payload[55:63], inverso=True))
        print("Zonas em bypass:", self.bits_para_numeros(payload[55:63]))
        print("Sirenes ligadas:", self.bits_para_numeros(payload[63:65]))

        # TODO interpretar mais campos
        print("*******************************************")
        print()

        self.despedida()

    def bits_para_numeros(self, octetos, inverso=False):
        lista = []
        for i, octeto in enumerate(octetos):
            for j in range(0, 8):
                bit = octeto & (1 << j)
                if (bit and not inverso) or (not bit and inverso):
                    lista.append("%d" % (1 + j + i * 8))
        return ", ".join(lista)
