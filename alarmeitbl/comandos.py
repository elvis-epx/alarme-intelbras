#!/usr/bin/env python3

import time
from .myeventloop.tcpclient import *
from .utils_proto import *

class DesativarCentral(TCPClientHandler, UtilsProtocolo):
    def __init__(self, ip_addr, cport, senha, tam_senha, particao, observer):
        super().__init__((ip_addr, cport))
        self.log_debug("Início comunicacao")
        self.conn_timeout = self.timeout("conn_timeout", 15, self.conn_timeout)
        self.senha = senha
        self.tam_senha = tam_senha
        self.particao = particao
        self.observer = observer

        self.dados_retorno = None
        # Se destruído com esse status, reporta erro fatal
        self.status = 2

    def destroyed_callback(self):
        if self.observer:
            self.observer.resultado(self.status, self.dados_retorno)

    def conn_timeout(self, task):
        self.status = 2
        self.log_info("Timeout conexao")
        self.destroy()

    def connection_callback(self, ok):
        self.conn_timeout.cancel()
        if not ok:
            self.status = 2
            self.log_info("Conexao falhou")
            # destroy() executado pelo chamador
            return
        self.envia_requisicao()

    def envia_requisicao(self):
        self.log_debug("Envio de requisicao")
        params = []
        if self.particao is not None:
            params = [0x40 + self.particao]
        pct = self.encode_isecnet([0x44], self.senha, self.tam_senha, params)
        self.log_debug("Dados enviados", self.hexprint(pct))
        self.send(pct)
        self.conn_timeout.restart()

    def recv_callback(self, latest):
        self.log_debug("Dados recebidos", self.hexprint(latest))
        # Parse, etc.
        # pct, self.recv_buf = self.recv_buf[:compr], self.recv_buf[compr:]
        self.conn_timeout.cancel()
        self.status = 0
        self.destroy()
