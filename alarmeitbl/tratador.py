#!/usr/bin/env python3

import datetime, os, shlex

from .myeventloop.tcpserver import *
from .utils_proto import *

class Tratador(TCPServerHandler, UtilsProtocolo):
    backoff_minimo = 0.125
    recuo_backoff_minimo = 1.0 # Deve ser bem maior que RTT esperado

    eventos_contact_id = {
        100: {'*': "Emergencia medica"},
        110: {'*': "Alarme de incendio"},
        120: {'*': "Panico"},
        121: {'*': "Ativacao/desativacao sob coacao"},
        122: {'*': "Panico silencioso"},
        130: {
            'aber': "Disparo de zona {zona}",
            'rest': "Restauracao de zona {zona}"
             },
        133: {'*': "Disparo de zona 24h {zona}"},
        146: {'*': "Disparo silencioso {zona}"},
        301: {
            'aber': "Falta de energia AC",
            'rest': "Retorno de energia AC"
             },
        342: {
             'aber': "Falta de energia AC em componente sem fio {zona}",
             'rest': "Retorno energia AC em componente sem fio {zona}"
             },
        302: {
            'aber': "Bateria do sistema baixa",
            'rest': "Recuperacao bateria do sistema baixa"
             },
        305: {'*': "Reset do sistema"},
        306: {'*': "Alteracao programacao"},
        311: {
            'aber': "Bateria ausente",
            'rest': "Recuperacao bateria ausente"
             },
        351: {
            'aber': "Corte linha telefonica",
            'rest': "Restauro linha telefonica"
             },
        354: {'*': "Falha ao comunicar evento"},
        147: {
            'aber': "Falha de supervisao {zona}",
            'rest': "Recuperacao falha de supervisao {zona}"
             },
        145: {
             'aber': "Tamper em dispositivo expansor {zona}",
             'rest': "Restauro tamper em dispositivo expansor {zona}"
              },
        383: {
              'aber': "Tamper em sensor {zona}",
              'rest': "Restauro tamper em sensor {zona}"
              },
        384: {
            'aber': "Bateria baixa em componente sem fio {zona}",
            'rest': "Recuperacao bateria baixa em componente sem fio {zona}"
             },
        401: {
             'rest': "Ativacao manual P{particao}",
             'aber': "Desativacao manual P{particao}"
             },
        403: {
             'rest': "Ativacao automatica P{particao}",
             'aber': "Desativacao automatica P{particao}"
             },
        404: {
            'rest': "Ativacao remota P{particao}",
            'aber': "Desativacao remota P{particao}",
             },
        407: {
            'rest': "Ativacao remota app P{particao}",
            'aber': "Desativacao remota app P{particao}",
             },
        408: {'*': "Ativacao por uma tecla P{particao}"},
        410: {'*': "Acesso remoto"},
        461: {'*': "Senha incorreta"},
        533: {
             'aber': "Adicao de zona {zona}",
             'rest': "Remocao de zona {zona}"
             },
        570: {
             'aber': "Bypass de zona {zona}",
             'rest': "Cancel bypass de zona {zona}"
             },
        602: {'*': "Teste periodico"},
        621: {'*': "Reset do buffer de eventos"},
        601: {'*': "Teste manual"},
        616: {'*': "Solicitacao de manutencao"},
        422: {
            'aber': "Acionamento de PGM {zona}",
            'rest': "Desligamento de PGM {zona}"
             },
        625: {'*': "Data e hora reiniciados"}
    }

    def __init__(self, addr, sock):
        super().__init__(addr, sock)

        self.log_info("inicio")
        self.backoff = Tratador.backoff_minimo

        self.ignorar = False
        self.central_identificada = False
        self.to_ident = self.timeout("ident", 120, self.timeout_identificacao)
        self.to_comm = self.timeout("comm", 600, self.timeout_comunicacao)
        self.to_processa = None
        self.to_incompleta = None
        self.to_backoff = None

        self.ip_addr = addr[0]

        if not Tratador.valida_maxconn():
            self.log_info("numero maximo de conexoes atingido - conexao ignorada")
            self.ignorar = True

    def timeout_comunicacao(self, _):
        self.log_info("timeout de comunicacao")
        self.destroy()

    def timeout_msgincompleta(self, _):
        self.log_warn("timeout de mensagem incompleta, buf =", self.hexprint(self.recv_buf))
        self.destroy()

    def timeout_identificacao(self, _):
        self.log_warn("timeout de identificacao")
        self.destroy()

    def _envia(self, resposta):
        self.send(bytearray(resposta))
        self.log_debug("enviada resposta", self.hexprint(resposta))

    def enquadrar(self, dados):
        dados = [len(dados)] + dados
        return dados + [ self.checksum(dados) ]

    def envia_longo(self, resposta):
        resposta = self.enquadrar(resposta)
        self._envia(resposta)

    def envia_curto(self, resposta):
        self._envia(resposta)

    def recv_callback(self, _):
        if self.ignorar:
            self.recv_buf = []
            return

        self.log_debug("evento")
        self.log_debug("buf =", self.hexprint(self.recv_buf))
        self.to_comm.restart()
        if not self.to_processa:
            self.to_processa = self.timeout("proc_msg", self.backoff, self.processar_msg)

    def shutdown_callback(self):
        self.log_info("fechada")
        super().shutdown_callback() # impl padrão = fechar

    def send_callback(self):
        self.log_debug("envio dados")
        super().send_callback()

    def processar_msg(self, _):
        self.to_processa = None
        msg_aceita, msgs_pendentes = self.consome_msg()
        if msg_aceita:
            self.avancar_backoff()
        if msgs_pendentes:
            self.to_processa = self.timeout("proc_msg", self.backoff, self.processar_msg)

    def consome_msg(self):
        if self.consome_frame_curto() or self.consome_frame_longo():
            # Processou uma mensagem
            if self.to_incompleta:
                self.to_incompleta.cancel()
                self.to_incompleta = None
            return True, not not self.recv_buf

        if self.recv_buf:
            # Mensagem incompleta no buffer
            if not self.to_incompleta:
                self.to_incompleta = self.timeout("msgincompleta", 60, self.timeout_msgincompleta)
        return False, False

    def avancar_backoff(self):
        self.backoff *= 2 # Backoff exponencial
        self.log_debug("backoff aumentado para %f" % self.backoff)

        if self.to_backoff:
            self.to_backoff.cancel()
            self.to_backoff = None

        self.to_backoff = self.timeout("recuar_backoff",
            max(Tratador.recuo_backoff_minimo, self.backoff * 2),
            self.recuar_backoff)

    def recuar_backoff(self, _):
        self.to_backoff = None

        self.backoff /= 2
        self.backoff = max(self.backoff, Tratador.backoff_minimo)
        self.log_debug("backoff reduzido para %f" % self.backoff)

        if self.backoff > Tratador.backoff_minimo:
            self.to_backoff = self.timeout("recuar_backoff",
                max(Tratador.recuo_backoff_minimo, self.backoff * 2),
                self.recuar_backoff)

    def consome_frame_curto(self):
        if self.recv_buf and self.recv_buf[0] == 0xf7:
            self.recv_buf = self.recv_buf[1:]
            self.log_debug("heartbeat da central")
            resposta = [0xfe]
            self.envia_curto(resposta)
            return True
        return False

    def consome_frame_longo(self):
        if len(self.recv_buf) < 2:
            return False

        esperado = self.recv_buf[0] + 2 # comprimento + dados + checksum
        if len(self.recv_buf) < esperado:
            return False

        rawmsg = self.recv_buf[:esperado]
        self.recv_buf = self.recv_buf[esperado:]

        # checksum de pacote sufixado com checksum resulta em 0
        if self.checksum(rawmsg) != 0x00:
            self.log_warn("checksum errado, rawmsg =", self.hexprint(rawmsg))
            return True

        # Mantém checksum no final pois, em algumas mensagens, o último octeto
        # calcula como checksum mas tem outro significado (e.g. 0xb5)
        msg = rawmsg[1:]

        if not msg:
            self.log_warn("mensagem nula")
            return True

        tipo = msg[0]
        msg = msg[1:]

        if tipo == 0x80:
            self.solicita_data_hora(msg)
        elif tipo == 0x94:
            self.identificacao_central(msg)
        elif tipo == 0xb0:
            self.evento_alarme(msg, False)
        elif tipo == 0xb5:
            self.evento_alarme(msg, True)
        else:
            self.log_warn("solicitacao desconhecida %02x payload =" % tipo, self.hexprint(msg))
            self.resposta_generica(msg)
        return True

    def resposta_generica(self, msg):
        resposta = [0xfe]
        self.envia_curto(resposta)

    def identificacao_central(self, msg):
        resposta = [0xfe]

        if len(msg) != 7:
            self.log_warn("identificacao central: tamanho inesperado,", self.hexprint(msg))
            self.envia_curto(resposta)

        canal = msg[0] # 'E' (0x45)=Ethernet, 'G'=GPRS, 'H'=GPRS2
        conta = self.from_bcd(msg[1:3])
        macaddr = msg[3:6]
        macaddr_s = (":".join(["%02x" % i for i in macaddr])).lower()
        self.log_info("identificacao central conta %d mac %s" % (conta, macaddr_s))

        if not Tratador.valida_central(macaddr_s):
            self.log_info("central nao autorizada")
            self.ignorar = True
            return

        # Testa novamente maxconn pois há uma "janela" de tempo entre conexão e
        # identificação onde mais conexões podem ter sido aceitas
        if not Tratador.valida_maxconn():
            self.log_info("numero maximo de conexoes atingido - conexao ignorada")
            self.ignorar = True
            return

        self.central_identificada = True
        if self.to_ident:
            self.to_ident.cancel()
            self.to_ident = None

        self.envia_curto(resposta)

    def solicita_data_hora(self, msg):
        self.log_debug("solicitacao de data/hora pela central")
        agora = datetime.datetime.now()
        # proto: 0 = domingo; weekday(): 0 = segunda
        dow = (agora.weekday() + 1) % 7
        resposta = [ 0x80, self.bcd(agora.year - 2000), self.bcd(agora.month), self.bcd(agora.day), \
            self.bcd(dow), self.bcd(agora.hour), self.bcd(agora.minute), self.bcd(agora.second) ]
        self.envia_longo(resposta)

    def msg_para_gancho(self, *msg):
        now = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        msgw = now
        for item in msg:
            msgw += " "
            msgw += str(item)
        p = os.popen(Tratador.gancho_msg + " " + shlex.quote(msgw), 'w')
        p.close()

    def ev_para_gancho(self, codigo, particao, zona, qualificador):
        p = os.popen("%s %d %d %d %d" % (Tratador.gancho_ev, codigo, particao, zona, qualificador), 'w')
        p.close()

    def evento_alarme(self, msg, com_foto):
        compr = com_foto and 20 or 17
        if len(msg) != compr:
            self.log_warn("evento de alarme de tamanho inesperado,", self.hexprint(msg))
            resposta = [0xfe]
            self.envia_curto(resposta)
            return

        canal = msg[0] # 0x11 Ethernet IP1, 0x12 IP2, 0x21 GPRS IP1, 0x22 IP2
        contact_id = self.contact_id_decode(msg[1:5])
        tipo_msg = self.contact_id_decode(msg[5:7]) # 18 decimal = Contact ID
        qualificador = msg[7]
        codigo = self.contact_id_decode(msg[8:11])
        particao = self.contact_id_decode(msg[11:13])
        zona = self.contact_id_decode(msg[13:16])
        if com_foto:
            checksum = msg[16] # truque do protocolo de reposicionar o checksum
            indice = msg[17] * 256 + msg[18]
            nr_fotos = msg[19]

        self.ev_para_gancho(codigo, particao, zona, qualificador)

        desconhecido = True
        if tipo_msg == 18 and codigo in Tratador.eventos_contact_id:
            if qualificador == 1:
                squalif = "aber"
                if squalif not in Tratador.eventos_contact_id[codigo]:
                    squalif = "*"
            elif qualificador == 3:
                squalif = "rest"
                if squalif not in Tratador.eventos_contact_id[codigo]:
                    squalif = "*"
            else:
                squalif = "*"

            if squalif in Tratador.eventos_contact_id[codigo]:
                desconhecido = False
                scodigo = Tratador.eventos_contact_id[codigo][squalif]
                fotos = ""
                if com_foto:
                    fotos = "(com fotos, i=%d n=%d)" % (indice, nr_fotos)
                descricao_humana = scodigo.format(zona=zona, particao=particao)
                self.log_info(descricao_humana, fotos)
                self.msg_para_gancho(descricao_humana, fotos)

                if com_foto:
                    for n in range(0, nr_fotos):
                        Tratador.tratador_de_fotos.enfileirar(self.ip_addr, indice, n)

        if desconhecido:
            msg = "Evento de alarme canal %02x contact_id %d tipo %d qualificador %d " \
                  "codigo %d particao %d zona %d" % \
                  (canal, contact_id, tipo_msg, qualificador, codigo, particao, zona)
            self.log_info(msg)
            self.msg_para_gancho(msg)

        resposta = [0xfe]
        self.envia_curto(resposta)
