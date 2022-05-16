#!/usr/bin/env python3

import sys, socket, time, select, datetime

def llog(*msg):
    now = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    print(now, *msg, flush=True)

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
            llog("valor contact id invalido", dados)
            return -1
        posicao *= 10
    return numero

def bcd(n):
    if n > 99 or n < 0:
        llog("valor invalido para BCD:", n)
        return 0
    return ((n // 10) << 4) + (n % 10)

class Tratador:
    tratadores = {}

    @staticmethod
    def lista_de_sockets():
        return [ tratador.sock for tratador in Tratador.tratadores.values() ]

    @staticmethod
    def pelo_socket(sock):
        if sock.fileno() in Tratador.tratadores:
            return Tratador.tratadores[sock.fileno()]
        return None

    @staticmethod
    def ping_all():
        for tratador in list(Tratador.tratadores.values()):
            tratador.ping()

    def __init__(self, sock, addr):
        Tratador.tratadores[sock.fileno()] = self
        self.sock = sock
        self.buf = []
        # Timeout de mensagem imcompleta
        # Corre apenas quando buffer contém mensagem incompleta
        self.msg_timeout = 0
        # Timeout de envio de identificação pela central
        # Corre apenas uma vez
        self.id_timeout = time.time() + 120
        # Timeout de comunicacao em geral
        # Sempre correndo, resetado a cada evento()
        self.comm_timeout = time.time() + 600
        self.log("Inicio", addr)

    def ping(self):
        if self.msg_timeout and time.time() > self.msg_timeout:
            self.log("Timeout de mensagem incompleta")
            self.encerrar()
        elif self.id_timeout and time.time() > self.id_timeout:
            self.log("Timeout de identificacao")
            self.encerrar()
        elif time.time() > self.comm_timeout:
            self.log("Timeout de comunicacao")
            self.encerrar()

    def log(self, *msg):
        llog("conn %d:" % self.sock.fileno(), *msg)

    def _envia(self, resposta):
        try:
            self.sock.sendall(bytearray(resposta))
            self.log("enviada resposta", ["%02x" % i for i in resposta])
        except socket.error as err:
            self.log("excecao ao enviar", err)
            self.encerrar()

    def envia_longo(self, resposta):
        resposta = enquadrar(resposta)
        self._envia(resposta)

    def envia_curto(self, resposta):
        self._envia(resposta)

    def encerrar(self):
        self.log("encerrando")
        del Tratador.tratadores[self.sock.fileno()]
        try:
            self.sock.close()
        except socket.error:
            pass
        self.sock = None

    def evento(self):
        self.log("Evento")
        try:
            data = self.sock.recv(1024)
        except socket.error as err:
            self.log("excecao ao ler", err)
            self.encerrar()
            return

        if not data:
            self.log("fechada")
            self.encerrar()
            return

        self.buf += [x for x in data]
        self.log("Buf atual ", ["%02x" % i for i in self.buf])

        # Reseta timeout de comunicacao
        self.comm_timeout = time.time() + 600

        # Inicia timer de mensagem incompleta
        if not self.msg_timeout:
            self.msg_timeout = time.time() + 120

        self.determina_msg()

    # FIXME rate limiting (sintoma de problema de parse/resposta)
    def determina_msg(self):
        while self.consome_frame_curto() or self.consome_frame_longo():
            pass

        if not self.buf:
            # Nenhuma mensagem incompleta, inibe timeout respectivo
            self.msg_timeout = 0

    def consome_frame_curto(self):
        if self.buf and self.buf[0] == 0xf7:
            self.buf = self.buf[1:]
            self.log("Heartbeat")
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
            self.log("checksum errado - fatal")
            self.encerrar()
            return True

        msg = rawmsg[1:-1]

        if not msg:
            self.log("Mensagem nula")
            return True

        tipo = msg[0]
        msg = msg[1:]

        if tipo == 0x80:
            self.solicita_data_hora(msg)
        elif tipo == 0x94:
            self.identificacao_central(msg)
        elif tipo == 0xb0:
            self.evento_alarme(msg)
        else:
            self.log("Solicitacao desconhecida %02x" % tipo)
            self.resposta_generica(msg)
        return True

    def resposta_generica(self, msg):
        resposta = [0xfe]
        self.envia_curto(resposta)

    # FIXME verificar se a central é a esperada
    def identificacao_central(self, msg):
        self.log("Envio identificacao pela central")
        self.id_timeout = 0
        resposta = [0xfe]
        self.envia_curto(resposta)

    def solicita_data_hora(self, msg):
        self.log("Solicitacao de data/hora pela central")
        agora = datetime.datetime.now()
        # proto: 0 = domingo; weekday(): 0 = segunda
        dow = (agora.weekday() + 1) % 7
        resposta = [ 0x80, bcd(agora.year - 2000), bcd(agora.month), bcd(agora.day), \
            bcd(dow), bcd(agora.hour), bcd(agora.minute), bcd(agora.second) ]
        self.envia_longo(resposta)

    def evento_alarme(self, msg):
        if len(msg) != 16:
            self.log("Evento de alarme de tamanho inesperado")
            resposta = [0xfe]
            self.envia_curto(resposta)
            return

        self.log(msg)
        canal = msg[0] # 0x11 Ethernet IP1, 0x12 IP2, 0x21 GPRS IP1, 0x22 IP2
        contact_id = contact_id_decode(msg[1:5])
        tipo_msg = contact_id_decode(msg[5:7])
        qualificador = msg[7]
        codigo = contact_id_decode(msg[8:11])
        particao = contact_id_decode(msg[11:13])
        zona = contact_id_decode(msg[13:16])

        self.log("Evento de alarme canal %02x contact_id %d tipo %d qualificador %d "
                    "codigo %d particao %d zona %d" % \
                    (canal, contact_id, tipo_msg, qualificador, codigo, particao, zona))

        resposta = [0xfe]
        self.envia_curto(resposta)


serverfd = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
HOST, PORT = "0.0.0.0", 9009 + int(sys.argv[1])
serverfd.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
serverfd.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEPORT, 1)
# serverfd.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, 0)
serverfd.bind((HOST, PORT))
llog("Porta %d" % PORT)

serverfd.listen(5)

# Loop de eventos
while True:
    sockets = [serverfd]
    sockets += Tratador.lista_de_sockets()
    rd, wr, ex = select.select(sockets, [], [], 60)

    if serverfd in rd:
        try:
            cliente_sock, addr = serverfd.accept()
            tratador = Tratador(cliente_sock, "%s:%d" % addr)
        except socket.error:
            pass
    elif rd:
        tratador = Tratador.pelo_socket(rd[0])
        if not tratador:
            llog("(bug?) evento em socket sem tratador")
            rd[0].close()
        else:
            tratador.evento()
    else:
        llog(".")

    Tratador.ping_all()
