#!/usr/bin/env python3

import time
from .myeventloop.tcpclient import *
from .utils_proto import *

# Envio de comandos diretamente do cliente para a central de alarme
# usando o novo protocolo ISECNet2 (o mesmo do download de fotos)

class ComandarCentral(TCPClientHandler, UtilsProtocolo):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__((ip_addr, cport))
        self.log_info("Inicio")
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
        self.status = 1
        self.log_info("Timeout")
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

        self.log_info("Autenticacao ok")
        self.envia_comando_in()

    def envia_comando(self, cmd, payload, tratador_in):
        pct = self.pacote_isecnet2(cmd, payload)
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

        if cmd != self.cmd:
            self.log_info("Resposta inesperada %04x" % cmd)
            self.destroy()
            return

        self.tratador_in(payload)

    def despedida(self):
        self.log_debug("Despedindo")
        pct = self.pacote_isecnet2_bye()
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
            self.log_info("NAK motivo %02x" % motivo)
        self.destroy()


class DesativarCentral(ComandarCentral):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra)
        self.particao = extra[0]

    def envia_comando_in(self):
        # byte 1: particao (0x01 = 1, 0xff = todas ou sem particao)
        # byte 2: 0x00 desarmar, 0x01 armar, 0x02 stay
        if self.particao is None:
            payload = [ 0xff, 0x00 ]
        else:
            payload = [ self.particao, 0x00 ]
        self.envia_comando(0x401e, payload, self.resposta_comando_in)

    def resposta_comando_in(self, payload):
        # FIXME interpretar payload
        self.despedida()


class AtivarCentral(ComandarCentral):
    def __init__(self, observer, ip_addr, cport, senha, tam_senha, extra):
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra)
        self.particao = extra[0]

    def envia_comando_in(self):
        # byte 1: particao (0x01 = 1, 0xff = todas ou sem particao)
        # byte 2: 0x00 desarmar, 0x01 armar, 0x02 stay
        if self.particao is None:
            payload = [ 0xff, 0x01 ]
        else:
            payload = [ self.particao, 0x01 ]
        self.envia_comando(0x401e, payload, self.resposta_comando_in)

    def resposta_comando_in(self, payload):
        # FIXME interpretar payload
        self.despedida()
