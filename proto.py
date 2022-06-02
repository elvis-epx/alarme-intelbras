#!/usr/bin/env python3

import sys, socket, time, datetime

from myeventloop import Timeout, Handler, EventLoop, Log, LOG_INFO, LOG_DEBUG

Log.set_level(LOG_INFO)

def hexprint(buf):
    return ", ".join(["%02x" % n for n in buf])

# Calcula checksum de frame longo
# Presume que "dados" contém o byte de comprimento mas não contém o byte de checksum
def checksum(dados):
    checksum = 0
    for n in dados:
        checksum ^= n
    checksum ^= 0xff
    checksum &= 0xff
    return checksum

# Verifica checksum de frame longo
# Presume um frame completo, com checksum no último byte
def verificar(dados):
    return checksum(dados[:-1]) == dados[-1]

# Enquadra mensagem no formato quadro longo
# Adiciona comprimento no início e checksum no final
def enquadrar(dados):
    dados = [len(dados)] + dados
    return dados + [ checksum(dados) ]

# Decodifica número no formato "Contact ID"
# Retorna -1 se aparenta estar corrompido
def contact_id_decode(dados):
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
            Log.warn("valor contact id invalido", hexprint(dados))
            return -1
        posicao *= 10
    return numero

def bcd(n):
    if n > 99 or n < 0:
        Log.warn("valor invalido para BCD: %02x" % n)
        return 0
    return ((n // 10) << 4) + (n % 10)

def from_bcd(dados):
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
             'rest': "Ativacao manual",
             'aber': "Desativacao manual"
             },
        403: {
             'rest': "Ativacao automatica",
             'aber': "Desativacao automatica"
             },
        404: {
            'rest': "Ativacao remota",
            'aber': "Desativacao remota",
             },
        407: {
            'rest': "Ativacao remota II",
            'aber': "Desativacao remota II",
             },
        408: {'*': "Ativacao por uma tecla"},
        410: {'*': "Acesso remoto"},
        461: {'*': "Senha incorreta"},
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

class TratadorConexao(Handler):
    backoff_minimo = 0.125
    recuo_backoff_minimo = 1.0 # Deve ser bem maior que RTT esperado

    def __init__(self, addr, sock):
        super().__init__("%s:%d" % addr, sock, socket.error)
        self.log_info("inicio")

        self.buf = []
        self.backoff = TratadorConexao.backoff_minimo

        # FIXME classe de solicitações na direção receptor -> central,
        # instrumentação para envio, recepção e pendência

        self.to_ident = Timeout(self, "ident", 120, self.timeout_identificacao)
        self.to_comm = Timeout(self, "comm", 600, self.timeout_comunicacao)
        self.to_hb = Timeout(self, "hb", 3600, self.heartbeat)
        self.to_processa = None
        self.to_incompleta = None
        self.to_backoff = None

    def heartbeat(self):
        self.log_info("ainda ativa")
        self.to_hb = Timeout(self, "hb", 3600, self.heartbeat)

    def timeout_comunicacao(self):
        self.log_info("timeout de comunicacao")
        self.destroy()

    def timeout_msgincompleta(self):
        self.log_warn("timeout de mensagem incompleta, buf =", hexprint(self.buf))
        self.destroy()

    def timeout_identificacao(self):
        self.log_warn("timeout de identificacao")
        self.destroy()

    def _envia(self, resposta):
        try:
            self.fd.sendall(bytearray(resposta))
            self.log_debug("enviada resposta", hexprint(resposta))
        except socket.error as err:
            self.log_debug("excecao ao enviar", err)
            self.destroy()

    def envia_longo(self, resposta):
        resposta = enquadrar(resposta)
        self._envia(resposta)

    def envia_curto(self, resposta):
        self._envia(resposta)

    def read_callback(self):
        self.log_debug("evento")
        try:
            data = self.fd.recv(1024)
        except socket.error as err:
            self.log_debug("excecao ao ler", err)
            self.destroy()
            return

        if not data:
            self.log_debug("fechada")
            self.destroy()
            return

        self.buf += [x for x in data]
        self.log_debug("buf =", hexprint(self.buf))

        if self.to_comm:
            self.to_comm.cancel()
        self.to_comm = Timeout(self, "comm", 600, self.timeout_comunicacao)

        if not self.to_processa:
            self.to_processa = Timeout(self, "proc_msg", self.backoff, self.processar_msg)

    def processar_msg(self):
        self.to_processa = None
        msg_aceita, msgs_pendentes = self.consome_msg()
        if msg_aceita:
            self.avancar_backoff()
        if msgs_pendentes:
            self.to_processa = Timeout(self, "proc_msg", self.backoff, self.processar_msg)

    def consome_msg(self):
        if self.consome_frame_curto() or self.consome_frame_longo():
            # Processou uma mensagem
            if self.to_incompleta:
                self.to_incompleta.cancel()
                self.to_incompleta = None
            return True, not not self.buf

        if self.buf:
            # Mensagem incompleta no buffer
            if not self.to_incompleta:
                self.to_incompleta = Timeout(self, "msgincompleta", 60, self.timeout_msgincompleta)
        return False, False

    def avancar_backoff(self):
        self.backoff *= 2 # Backoff exponencial
        self.log_debug("backoff aumentado para %f" % self.backoff)

        if self.to_backoff:
            self.to_backoff.cancel()
            self.to_backoff = None

        self.to_backoff = Timeout(self, "recuar_backoff",
            max(TratadorConexao.recuo_backoff_minimo, self.backoff * 2),
            self.recuar_backoff)

    def recuar_backoff(self):
        self.to_backoff = None

        self.backoff /= 2
        self.backoff = max(self.backoff, TratadorConexao.backoff_minimo)
        self.log_debug("backoff reduzido para %f" % self.backoff)

        if self.backoff > TratadorConexao.backoff_minimo:
            self.to_backoff = Timeout(self, "recuar_backoff",
                max(TratadorConexao.recuo_backoff_minimo, self.backoff * 2),
                self.recuar_backoff)

    def consome_frame_curto(self):
        if self.buf and self.buf[0] == 0xf7:
            self.buf = self.buf[1:]
            self.log_debug("heartbeat da central")
            resposta = [0xfe]
            self.envia_curto(resposta)
            return True
        return False

    def consome_frame_longo(self):
        if len(self.buf) < 2:
            return False

        esperado = self.buf[0] + 2 # comprimento + dados + checksum
        if len(self.buf) < esperado:
            return False

        rawmsg = self.buf[:esperado]
        self.buf = self.buf[esperado:]

        if not verificar(rawmsg):
            self.log_warn("checksum errado - fatal, rawmsg =", hexprint(rawmsg))
            self.destroy()
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
            self.evento_alarme(msg)
        elif tipo == 0xb5:
            self.evento_alarme_foto(msg)
        else:
            self.log_warn("solicitacao desconhecida %02x payload =" % tipo, hexprint(msg))
            self.resposta_generica(msg)
        return True

    def resposta_generica(self, msg):
        resposta = [0xfe]
        self.envia_curto(resposta)

    # FIXME verificar se a central é a esperada
    def identificacao_central(self, msg):
        if len(msg) != 7:
            self.log_warn("identificacao central: tamanho inesperado,", hexprint(msg))
        else:
            canal = msg[0] # 'E' (0x45)=Ethernet, 'G'=GPRS, 'H'=GPRS2
            conta = from_bcd(msg[1:3])
            macaddr = msg[3:6]
            macaddr_s = ":".join(["%02x" % i for i in macaddr])
            self.log_info("identificacao central conta %d mac %s" % (conta, macaddr_s))
            if self.to_ident:
                self.to_ident.cancel()
                self.to_ident = None

        resposta = [0xfe]
        self.envia_curto(resposta)

    def solicita_data_hora(self, msg):
        self.log_debug("solicitacao de data/hora pela central")
        agora = datetime.datetime.now()
        # proto: 0 = domingo; weekday(): 0 = segunda
        dow = (agora.weekday() + 1) % 7
        resposta = [ 0x80, bcd(agora.year - 2000), bcd(agora.month), bcd(agora.day), \
            bcd(dow), bcd(agora.hour), bcd(agora.minute), bcd(agora.second) ]
        self.envia_longo(resposta)

    def evento_alarme_foto(self, msg):
        if len(msg) != 20:
            self.log_warn("evento de alarme F de tamanho inesperado,", hexprint(msg))
            resposta = [0xfe]
            self.envia_curto(resposta)
            return

        canal = msg[0] # 0x11 Ethernet IP1, 0x12 IP2, 0x21 GPRS IP1, 0x22 IP2
        contact_id = contact_id_decode(msg[1:5])
        tipo_msg = contact_id_decode(msg[5:7])
        qualificador = msg[7]
        codigo = contact_id_decode(msg[8:11])
        particao = contact_id_decode(msg[11:13])
        zona = contact_id_decode(msg[13:16])
        checksum = msg[16] # truque do protocolo de reposicionar o checksum
        indice = msg[17] * 256 + msg[18]
        nr_fotos = msg[19]

        self.log_info("Evento de alarme F canal %02x contact_id %d tipo %d qualificador %d "
                      "codigo %d particao %d zona %d fotos %d (%d)" % \
                       (canal, contact_id, tipo_msg, qualificador, codigo, particao, zona,
                       indice, nr_fotos))
        # FIXME como ler dados do evento apontado no indice?

        resposta = [0xfe]
        self.envia_curto(resposta)

    def evento_alarme(self, msg):
        if len(msg) != 17:
            self.log_warn("evento de alarme de tamanho inesperado,", hexprint(msg))
            resposta = [0xfe]
            self.envia_curto(resposta)
            return

        canal = msg[0] # 0x11 Ethernet IP1, 0x12 IP2, 0x21 GPRS IP1, 0x22 IP2
        contact_id = contact_id_decode(msg[1:5])
        tipo_msg = contact_id_decode(msg[5:7]) # 18 decimal = Contact ID
        qualificador = msg[7]
        codigo = contact_id_decode(msg[8:11])
        particao = contact_id_decode(msg[11:13])
        zona = contact_id_decode(msg[13:16])

        desconhecido = True
        if tipo_msg == 18 and codigo in eventos_contact_id:
            if qualificador == 1:
                squalif = "aber"
                if squalif not in eventos_contact_id[codigo]:
                    squalif = "*"
            elif qualificador == 3:
                squalif = "rest"
                if squalif not in eventos_contact_id[codigo]:
                    squalif = "*"
            else:
                squalif = "*"

            if squalif in eventos_contact_id[codigo]:
                desconhecido = False
                scodigo = eventos_contact_id[codigo][squalif]
                self.log_info(scodigo.format(zona=zona, particao=particao))

        if desconhecido:
            self.log_info("Evento de alarme canal %02x contact_id %d tipo %d qualificador %d "
                          "codigo %d particao %d zona %d" % \
                          (canal, contact_id, tipo_msg, qualificador, codigo, particao, zona))

        resposta = [0xfe]
        self.envia_curto(resposta)


# Socket que aceita conexões 

serverfd = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
HOST, PORT = "0.0.0.0", 9009 + int(sys.argv[1])
serverfd.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
serverfd.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEPORT, 1)
# serverfd.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, 0)
serverfd.bind((HOST, PORT))
serverfd.listen(5)
Log.info("Porta %d" % PORT)

class NovaConexao(Handler):
    def __init__(self, fd):
        super().__init__("listener", fd, socket.error)

    def read_callback(self):
        try:
            client_sock, addr = self.fd.accept()
            TratadorConexao(addr, client_sock)
        except socket.error:
            return

accepthandler = NovaConexao(serverfd)

def heartbeat():
    Log.info("receptor ok")
    Timeout(None, "heartbeat", 3600, heartbeat)

Timeout(None, "heartbeat", 60, heartbeat)

class MyEventLoop(EventLoop):
    def __init__(self):
        super().__init__()

MyEventLoop().loop()
