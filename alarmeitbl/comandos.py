#!/usr/bin/env python3

import time
from .utils_proto import *

class DesativarCentral(UtilsProtocolo):
    def __init__(self, senha, tam_senha, particao, observer):
        self.senha = senha
        self.tam_senha = tam_senha
        self.particao = particao
        self.observer = observer

    def gerar(self):
        params = []
        if self.particao is not None:
            params = [0x40 + self.particao]
        pct = self.encode_isecmobile([0x44], self.senha, self.tam_senha, params)
        self.log_debug("Dados enviados", self.hexprint(pct))
        return pct
