#!/usr/bin/env python3

import time
from .utils_proto import *
from .comandos import ComandarCentral

# Agente que obtem fotos de um evento de sensor com câmera

class ObtemFotosDeEvento(ComandarCentral):
    def __init__(self, ip_addr, cport, indice, nrfoto, senha, tam_senha, observer, folder):
        extra = [indice, nrfoto]
        super().__init__(observer, ip_addr, cport, senha, tam_senha, extra)
        self.log_info("Iniciando obtencao de foto %d:%d" % (indice, nrfoto))
        self.indice = indice
        self.nrfoto = nrfoto
        self.arquivo = ""
        self.folder = folder

        # Se destruído com esse status, reporta erro fatal
        self.status = 2

        self.tratador = None

    # override completo
    def destroyed_callback(self):
        # Informa observador sobre status final da tarefa
        self.observer.resultado_foto(self.indice, self.nrfoto, \
                                     self.status, self.arquivo)

    def envia_comando_in(self):
        self.fragmento_corrente = 1 # Fragmento 1 sempre existe
        self.jpeg_corrente = []
        self.obtem_fragmento_foto()

    def obtem_fragmento_foto(self):
        self.log_debug("Conexao foto: obtendo fragmento %d" % self.fragmento_corrente)
        payload = self.be16(self.indice) + [ self.nrfoto, self.fragmento_corrente ]
        self.envia_comando(0x0bb0, payload, self.resposta_comando_in)

    def resposta_comando_in(self, payload):
        if len(payload) < 6:
            self.log_info("Conexao foto: resp frag muito curta")
            self.destroy()
            return

        self.log_debug("Conexao foto: resposta fragmento %d" % self.fragmento_corrente)

        indice = self.parse_be16(payload[0:2])
        foto = payload[2]
        nr_fotos = payload[3]
        fragmento = payload[4]
        nr_fragmentos = payload[5]
        fragmento_jpeg = payload[6:]

        if indice != self.indice:
            self.log_info("Conexao foto: indice invalido")
            self.destroy()
            return

        if foto != self.nrfoto:
            self.log_info("Conexao foto: nr foto invalida")
            self.destroy()
            return

        if fragmento != self.fragmento_corrente:
            self.log_info("Conexao foto: frag corrente invalido")
            self.destroy()
            return

        self.jpeg_corrente += fragmento_jpeg

        if fragmento < nr_fragmentos:
            self.fragmento_corrente += 1
            self.obtem_fragmento_foto()
            return

        self.log_info("Conexao foto: salvando imagem")
        self.arquivo = self.folder + "/" + \
                "imagem.%d.%d.%.6f.jpeg" % (indice, foto, time.time())
        f = open(self.arquivo, "wb")
        f.write(bytearray(self.jpeg_corrente))
        f.close()

        self.despedida()

    # Motivos NAK (nem todos se aplicam a download de fotos):
    # 00    Mensagem Ok (Por que NAK então? ACK = cmd 0xf0fe)
    # 01    Erro de checksum (daqui para baixo, todos são erros)
    # 02    Número de bytes da mensagem
    # 03    Número de bytes do parâmetro (payload)
    # 04    Parâmetro inexistente
    # 05    Indice parâmetro
    # 06    Valor máximo
    # 07    Valor mínimo
    # 08    Quantidade de campos
    # 09    Nibble 0-9
    # 0a    Nibble 1-a
    # 0b    Nibble 0-f
    # 0c    Nibble 1-f-ex-b-c
    # 0d    ASCII
    # 0e    29 de fevereiro
    # 0f    Dia inválido
    # 10    Mês inválido
    # 11    Ano inválido
    # 12    Hora inválida
    # 13    Minuto inválido
    # 14    Segundo inválido
    # 15    Tipo de comando inválido
    # 16    Tecla especial
    # 17    Número de dígitos
    # 18    Número de dígitos senha
    # 19    Senha incorreta (mas reportado na resposta da autenticação, não por NAK)
    # 1a    Partição inexistente
    # 1b    Usuário sem permissão na partição
    # 1c    Sem permissão programar
    # 1d    Buffer de recepção cheio
    # 1e    Sem permissão para desarmar
    # 1f    Necessária autenticação prévia
    # 20    Sem zonas habilitadas
    # 21    Sem permissão para comando
    # 22    Sem partições definidas
    # 23    Evento sem foto associada
    # 24    Índice foto inválido
    # 25    Fragmento foto inválido
    # 26    Sistema não particionado
    # 27    Zonas abertas
    # 28    Ainda gravando foto / transferindo do sensor (tente mais tarde)
    # 29    Acesso mobile desabilitado
    # 2a    Operação não permitida
    # 2b    Memória RF vazia
    # 2c    Memória RF ocupada
    # 2d    Senha repetida
    # 2e    Falha ativação/desativação
    # 2f    Sem permissão arme stay
    # 30    Desative a central
    # 31    Reset bloqueado
    # 32    Teclado bloqueado
    # 33    Recebimento de foto falhou
    # 34    Não conectado ao servidor
    # 35    Taclado sem permissão
    # 36    Partição sem zonas stay
    # 37    Sem permissão bypass
    # 38    Firmware corrompido
    # fe    Comando inválido
    # ff    Erro não especificado (não documentado mas observado se checksum ou tamanho pacote errado)
