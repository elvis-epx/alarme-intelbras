#!/usr/bin/env python3

import os
from myeventloop import Timeout, Log
from obtem_fotos import *

# Tratador de fotos obtidas via eventos 0xb5. Desacoplado do tratador 
# principal pois usa conexões separadas, e as fotos ficam armazenadas
# por tempo indeterminado na central, não sendo atreladas à conexão com
# o Receptor IP.
#
# Numa implementação futura os índices das fotos poderiam ser até
# armazenados num banco de dados local, para que não se percam quando
# o programa é reiniciado.

class TratadorDeFotos:
    def __init__(self, cport, senha, tam_senha):
        self.cport = cport
        self.senha = senha
        self.tam_senha = tam_senha
        self.fila = [] # [endereço IP, indice, nr. foto, tentativas restantes]
        self.task = None

    # Recebe nova foto de algum Tratador para a fila
    def enfileirar(self, ip_addr, indice, nrfoto):
        if self.tam_senha <= 0:
            return
        self.fila.append([ip_addr, indice, nrfoto, 10])
        if not self.task:
            # Fotos de sensor 8000 demoram para gravar (NAK 0x28 = foto não gravada)
            self.task = Timeout.new("trata_foto", 20, self.obtem_foto)

    # Reduz tempo de timeout (caso de uso: dlfoto)
    def imediato(self):
        self.task.reset(0.1)

    def obtem_foto(self, task):
        if not self.fila:
            self.task = None
            return

        ip_addr, indice, nrfoto, tentativas = self.fila[0]
        Log.info("tratador de fotos: obtendo %s:%d:%d tentativas %d" % \
                      (ip_addr, indice, nrfoto, tentativas))

        ObtemFotosDeEvento(ip_addr, self.cport, indice, nrfoto, \
                            self.senha, self.tam_senha, self)

    def msg_para_gancho_arquivo(self, arquivo):
        p = os.popen("./gancho_arquivo %s" % arquivo, 'w')
        p.close()

    # observer chamado quando ObtemFotosDeEvento finaliza
    def resultado_foto(self, ip_addr, cport, indice, nrfoto, status, arquivo):
        if status == 0:
            Log.info("Fotos indice %s:%d:%d: sucesso" % (ip_addr, indice, nrfoto))
            Log.info("Arquivo de foto %s" % arquivo)
            self.msg_para_gancho_arquivo(arquivo)
            del self.fila[0]
        elif status == 2:
            Log.info("Fotos indice %s:%d:%d: erro fatal" % (ip_addr, indice, nrfoto))
            del self.fila[0]
        else:
            self.fila[0][3] -= 1
            if self.fila[0][3] <= 0:
                Log.info("Fotos indice %s:%d:%d: tentativas esgotadas" % (ip_addr, indice, nrfoto))
                del self.fila[0]
            else:
                Log.info("Fotos indice %s:%d:%d: erro temporario" % (ip_addr, indice, nrfoto))

        self.task.restart()
