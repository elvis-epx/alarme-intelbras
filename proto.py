#!/usr/bin/env python3

import sys, socket, time, select, datetime

LOG_ERROR = 0
LOG_WARN = 1
LOG_INFO = 2
LOG_DEBUG = 3

log_level = LOG_INFO

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
            Tratador.llog2(LOG_WARN, "valor contact id invalido", hexprint(dados))
            return -1
        posicao *= 10
    return numero

def bcd(n):
    if n > 99 or n < 0:
        Tratador.llog2(LOG_WARN, "valor invalido para BCD: %02x" % n)
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

class Tratador:
    tratadores = {}
    last_id = 0
    timeout_heartbeat = time.time() + 60

    @staticmethod
    def llog2(level, *msg):
        if level <= log_level:
            now = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
            print(now, *msg, flush=True)
            Tratador.resetar_heartbeat()

    @staticmethod
    def lista_de_sockets():
        return [ tratador.sock for tratador in Tratador.tratadores.values() ]

    @staticmethod
    def pelo_socket(sock):
        for tratador in Tratador.tratadores.values():
            if sock is tratador.sock:
                return tratador
        return None

    @staticmethod
    def resetar_heartbeat():
        Tratador.timeout_heartbeat = time.time() + 3600

    @staticmethod
    def heartbeat_geral():
        if time.time() > Tratador.timeout_heartbeat:
            Tratador.llog2(LOG_INFO, "receptor ok")
            Tratador.resetar_heartbeat()

    @staticmethod
    def ping():
        for tratador in list(Tratador.tratadores.values()):
            if tratador._ping():
                break
        else:
            Tratador.heartbeat_geral()

    @staticmethod
    def proximo_timeout():
        timeout = Tratador.timeout_heartbeat - time.time()
        timeout_nome = "heartbeat"
        for tratador in Tratador.tratadores.values():
            candidate, nome = tratador._proximo_timeout()
            if nome and candidate < timeout:
                timeout = candidate
                timeout_nome = "%d:%s" % (tratador.conn_id, nome)
        return (max(0, timeout), timeout_nome)

    def _proximo_timeout(self):
        timeout = 9999999
        timeout_nome = None
        for nome, value in self.timeouts.items():
            candidate = value['timeout'] - time.time()
            if candidate < timeout:
                timeout = candidate
                timeout_nome = nome
        return (timeout, timeout_nome)

    def start_timeout(self, nome, timeout, callback):
        if nome in self.timeouts:
            self.cancel_timeout(nome)
        self.timeouts[nome] = { "rel_timeout": timeout, "callback": callback }
        self.timeouts[nome]["timeout"] = \
            time.time() + self.timeouts[nome]["rel_timeout"]
        self.log2(LOG_DEBUG, "+ timeout %s %d" % (nome, timeout))

    def cancel_timeout(self, nome):
        if nome in self.timeouts:
            del self.timeouts[nome]
            self.log2(LOG_DEBUG, "- timeout %s" % nome)

    def reset_timeout(self, nome):
        if nome in self.timeouts:
            self.timeouts[nome]["timeout"] = \
                time.time() + self.timeouts[nome]["rel_timeout"]
            self.log2(LOG_DEBUG, "@ timeout %s %d" % (nome, self.timeouts[nome]["rel_timeout"]))

    def __init__(self, sock, addr):
        Tratador.last_id += 1
        self.conn_id = Tratador.last_id
        Tratador.tratadores[self.conn_id] = self

        self.sock = sock
        self.buf = []
        self.timeouts = {}

        # FIXME classe de solicitações na direção receptor -> central,
        # instrumentação para envio, recepção e pendência

        self.start_timeout("identificacao", 120, self.timeout_identificacao)
        self.start_timeout("comunicacao", 600, self.timeout_comunicacao)
        self.start_timeout("heartbeat", 3600, self.heartbeat)

        self.log2(LOG_INFO, "inicio", addr)

    def _ping(self):
        for nome, value in self.timeouts.items():
            if time.time() > value['timeout']:
                if value['callback'](nome):
                    self.cancel_timeout(nome)
                else:
                    self.reset_timeout(nome)
                # Não tenta prosseguir pois self.timeouts pode ter mudado
                break
        else:
            return False
        return True

    def heartbeat(self, nome):
        self.log2(LOG_INFO, "ainda ativa")
        return False

    def timeout_comunicacao(self, nome):
        self.log2(LOG_INFO, "timeout de comunicacao")
        self.encerrar()
        return True

    def timeout_msgincompleta(self, nome):
        self.log2(LOG_WARN, "timeout de mensagem incompleta, buf =", hexprint(self.buf))
        self.encerrar()
        return True

    def timeout_identificacao(self, nome):
        self.log2(LOG_WARN, "timeout de identificacao")
        self.encerrar()
        return True

    def log2(self, level, *msg):
        Tratador.llog2(level, "conn %d:" % self.conn_id, *msg)

    def _envia(self, resposta):
        try:
            self.sock.sendall(bytearray(resposta))
            self.log2(LOG_DEBUG, "enviada resposta", hexprint(resposta))
        except socket.error as err:
            self.log2(LOG_DEBUG, "excecao ao enviar", err)
            self.encerrar()

    def envia_longo(self, resposta):
        resposta = enquadrar(resposta)
        self._envia(resposta)

    def envia_curto(self, resposta):
        self._envia(resposta)

    def encerrar(self):
        self.log2(LOG_INFO, "encerrando")
        del Tratador.tratadores[self.conn_id]
        try:
            self.sock.close()
        except socket.error:
            pass
        self.timeouts = {}

    def evento(self):
        self.log2(LOG_DEBUG, "evento")
        try:
            data = self.sock.recv(1024)
        except socket.error as err:
            self.log2(LOG_DEBUG, "excecao ao ler", err)
            self.encerrar()
            return

        if not data:
            self.log2(LOG_DEBUG, "fechada")
            self.encerrar()
            return

        self.buf += [x for x in data]
        self.log2(LOG_DEBUG, "buf =", hexprint(self.buf))

        self.reset_timeout("comunicacao")
        self.start_timeout("msgincompleta", 60, self.timeout_msgincompleta)

        self.determina_msg()

    # FIXME rate limiting (sintoma de problema de parse/resposta)
    def determina_msg(self):
        while self.consome_frame_curto() or self.consome_frame_longo():
            pass

        if not self.buf:
            self.cancel_timeout("msgincompleta")

    def consome_frame_curto(self):
        if self.buf and self.buf[0] == 0xf7:
            self.buf = self.buf[1:]
            self.log2(LOG_DEBUG, "heartbeat da central")
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
            self.log2(LOG_WARN, "checksum errado - fatal, rawmsg =", hexprint(rawmsg))
            self.encerrar()
            return True

        msg = rawmsg[1:-1]

        if not msg:
            self.log2(LOG_WARN, "mensagem nula")
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
            self.log2(LOG_WARN, "solicitacao desconhecida %02x payload =" % tipo, hexprint(msg))
            self.resposta_generica(msg)
        return True

    def resposta_generica(self, msg):
        resposta = [0xfe]
        self.envia_curto(resposta)

    # FIXME verificar se a central é a esperada
    def identificacao_central(self, msg):
        if len(msg) != 6:
            self.log2(LOG_WARN, "identificacao central: tamanho inesperado,", hexprint(msg))
        else:
            canal = msg[0] # 'E' (0x45)=Ethernet, 'G'=GPRS, 'H'=GPRS2
            conta = from_bcd(msg[1:3])
            macaddr = msg[3:]
            macaddr_s = ":".join(["%02x" % i for i in macaddr])
            self.log2(LOG_INFO, "identificacao central conta %d mac %s" % (conta, macaddr_s))
            self.cancel_timeout("identificacao")

        resposta = [0xfe]
        self.envia_curto(resposta)

    def solicita_data_hora(self, msg):
        self.log2(LOG_DEBUG, "solicitacao de data/hora pela central")
        agora = datetime.datetime.now()
        # proto: 0 = domingo; weekday(): 0 = segunda
        dow = (agora.weekday() + 1) % 7
        resposta = [ 0x80, bcd(agora.year - 2000), bcd(agora.month), bcd(agora.day), \
            bcd(dow), bcd(agora.hour), bcd(agora.minute), bcd(agora.second) ]
        self.envia_longo(resposta)

    def evento_alarme_foto(self, msg):
        if len(msg) != 19:
            self.log2(LOG_WARN, "evento de alarme F de tamanho inesperado,", hexprint(msg))
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
        checksum = msg[16] # FIXME verificar
        indice = msg[17] * 256 + msg[18]
        nr_fotos = msg[19]

        self.log2(LOG_INFO,
                    "Evento de alarme F canal %02x contact_id %d tipo %d qualificador %d "
                    "codigo %d particao %d zona %d fotos %d (%d)" % \
                    (canal, contact_id, tipo_msg, qualificador, codigo, particao, zona,
                    indice, nr_fotos))
        # FIXME como ler dados do evento apontado no indice?

        resposta = [0xfe]
        self.envia_curto(resposta)

    def evento_alarme(self, msg):
        if len(msg) != 16:
            self.log2(LOG_WARN, "evento de alarme de tamanho inesperado,", hexprint(msg))
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
                self.log2(LOG_INFO, scodigo.format(zona=zona, particao=particao))

        if desconhecido:
            self.log2(LOG_INFO,
                    "Evento de alarme canal %02x contact_id %d tipo %d qualificador %d "
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
Tratador.llog2(LOG_INFO, "Porta %d" % PORT)

serverfd.listen(5)

# Loop de eventos
while True:
    sockets = [serverfd]
    sockets += Tratador.lista_de_sockets()
    to, tonome = Tratador.proximo_timeout()
    Tratador.llog2(LOG_DEBUG, "Proximo timeout %d (%s)" % (to, tonome))
    rd, wr, ex = select.select(sockets, [], [], to)

    if serverfd in rd:
        try:
            cliente_sock, addr = serverfd.accept()
            tratador = Tratador(cliente_sock, "%s:%d" % addr)
        except socket.error:
            pass
    elif rd:
        tratador = Tratador.pelo_socket(rd[0])
        if not tratador:
            Tratador.llog2(LOG_WARN, "(bug?) evento em socket sem tratador")
            rd[0].close()
        else:
            tratador.evento()
    else:
        Tratador.ping()
