#!/usr/bin/env python3

import time
from .myeventloop.tcpclient import *
from .utils_proto import *

# Agente que obtem fotos de um evento de sensor com câmera

class ObtemFotosDeEvento(TCPClientHandler, UtilsProtocolo):
    def __init__(self, ip_addr, cport, indice, nrfoto, senha, tam_senha, observer):
        super().__init__((ip_addr, cport))
        self.log_info("Iniciando obtencao de foto %d:%d" % (indice, nrfoto))
        self.conn_timeout = self.timeout("conn_timeout", 15, self.conn_timeout)
        self.indice = indice
        self.nrfoto = nrfoto
        self.senha = senha
        self.tam_senha = tam_senha
        self.observer = observer
        self.arquivo = ""

        # Se destruído com esse status, reporta erro fatal
        self.status = 2

        self.tratador = None

    def destroyed_callback(self):
        # Informa observador sobre status final da tarefa
        self.observer.resultado_foto(self.indice, self.nrfoto, \
                                     self.status, self.arquivo)

    def conn_timeout(self, task):
        if self.status != 0:
            # reporta erro não-fatal, exceto se status = 0 (download completo)
            self.status = 1
        self.log_info("Timeout conexao foto")
        self.destroy()

    def connection_callback(self, ok):
        self.conn_timeout.cancel()
        if not ok:
            self.status = 1 # erro não-fatal
            self.log_info("Conexao foto falhou")
            # destroy() executado pelo chamador
            return
        self.autenticacao()

    # Cria um pacote válido do protocolo de obtenção de fotos
    def pacote_foto(self, cmd, payload):
        # ID da central, sempre zero
        dst_id = self.be16(0x0000)
        # ID nosso, pode ser qualquer número, devolvido nos pacotes de retorno
        # Possivelmente uma relíquia de canais seriais onde múltiplos receptores
        # ouvem as mensagens, e dst_id ajudaria a identificar o recipiente
        src_id = self.be16(0x8fff)
        length = self.be16(len(payload) + 2)
        cmd_enc = self.be16(cmd)
        pacote = dst_id + src_id + length + cmd_enc + payload
        pacote = pacote + [ self.checksum(pacote) ]
        return pacote

    # Cria um pacote de autenticação para o protocolo de fotos
    # No protocolo de fotos, a autenticação deve preceder o download
    def pacote_foto_auth(self, senha, tam_senha):
        # 0x02 software de monitoramento, 0x03 mobile app
        sw_type = [ 0x02 ]
        senha = self.contact_id_encode(senha, tam_senha)
        sw_ver = [ 0x10 ]  # nibble.nibble (0x10 = 1.0)
        payload = sw_type + senha + sw_ver
        return self.pacote_foto(0xf0f0, payload)

    def autenticacao(self):
        self.log_debug("Conexao foto: auth")
        pct = self.pacote_foto_auth(self.senha, self.tam_senha)
        self.send(pct)

        self.tratador = self.resposta_autenticacao
        self.conn_timeout.restart()

    def resposta_autenticacao(self, cmd, payload):
        if cmd == 0xf0fd:
            self.nak(payload)
            return

        if cmd != 0xf0f0:
            self.log_info("Conexao foto: resp inesperada %04x" % cmd)
            self.destroy()
            return

        if len(payload) != 1:
            self.log_info("Conexao foto: resp auth invalida")
            self.destroy()
            return

        resposta = payload[0]
        # Possíveis respostas:
        # 01 = senha incorreta
        # 02 = versão software incorreta
        # 03 = painel chamará de volta (?)
        # 04 = aguardando permissão de usuário (?)
        if resposta > 0:
            self.log_info("Conexao foto: auth falhou motivo %d" % resposta)
            self.destroy()
            return

        self.log_info("Conexao foto: autenticado")
        self.inicia_obtencao_fotos()

    def inicia_obtencao_fotos(self):
        # Fragmento 1 sempre existe
        self.obtem_fragmento_foto(1, [])

    # Cria um pacote de requisição de fragmento de foto
    def pacote_foto_req(self, indice, foto, fragmento):
        payload = self.be16(indice) + [ foto, fragmento ]
        return self.pacote_foto(0x0bb0, payload)

    def obtem_fragmento_foto(self, fragmento_corrente, jpeg_corrente):
        self.log_debug("Conexao foto: obtendo fragmento %d" % fragmento_corrente)
        pct = self.pacote_foto_req(self.indice, self.nrfoto, fragmento_corrente)
        self.send(pct)

        def tratador(cmd, payload):
            self.resposta_fragmento(cmd, payload, fragmento_corrente, jpeg_corrente)

        self.tratador = tratador
        self.conn_timeout.restart()

    def resposta_fragmento(self, cmd, payload, fragmento_corrente, jpeg_corrente):
        if cmd == 0xf0fd:
            self.nak(payload)
            return

        if cmd == 0xf0f7:
            # Código não documentado retornado pela central após lentidão
            # Possivelmente sinaliza central ocupada
            self.status = 1 # erro não-fatal
            self.destroy()
            return

        if cmd != 0x0bb0:
            self.log_info("Conexao foto: resp inesperada %04x" % cmd)
            self.destroy()
            return

        if len(payload) < 6:
            self.log_info("Conexao foto: resp frag muito curta")
            self.destroy()
            return

        self.log_debug("Conexao foto: resposta fragmento %d" % fragmento_corrente)

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

        if fragmento != fragmento_corrente:
            self.log_info("Conexao foto: frag corrente invalido")
            self.destroy()
            return

        jpeg_corrente += fragmento_jpeg

        if fragmento < nr_fragmentos:
            self.obtem_fragmento_foto(fragmento + 1, jpeg_corrente)
            return

        self.log_info("Conexao foto: salvando imagem")
        self.arquivo = "imagem.%d.%d.%.6f.jpeg" % (indice, foto, time.time())
        f = open(self.arquivo, "wb")
        f.write(bytearray(jpeg_corrente))
        f.close()

        self.despedida()

    # Cria um pacote de desconexão
    def pacote_foto_bye(self):
        return self.pacote_foto(0xf0f1, [])

    def despedida(self):
        self.log_debug("Conexao foto: despedindo")
        pct = self.pacote_foto_bye()
        self.send(pct)

        self.tratador = None
        # Reportar sucesso ao observador
        self.status = 0
        self.conn_timeout.restart()
        # Resposta esperada: central fechar conexão

    def nak(self, payload):
        if len(payload) != 1:
            self.log_info("Conexao foto: NAK invalido")
        else:
            # NAK comum = 0x28 significando que foto ainda não foi transferida do sensor
            # Por isso consideramos NAK um erro não-fatal, poderia refinar por motivo
            motivo = payload[0]
            self.log_info("Conexao foto: NAK motivo %02x" % motivo)
            self.status = 1 # erro não-fatal
        self.destroy()

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

    # Retorna o comprimento de um pacote, se houver um pacote completo no buffer
    # Se não, retorna 0
    def pacote_foto_completo(self, data):
        # Um pacote tem tamanho mínimo 9 (src_id, dst_id, len, cmd, checksum)
        if len(data) < 9:
            return 0
        compr = 6 + self.parse_be16(data[4:6]) + 1
        if len(data) < compr:
            return 0
        return compr

    # Consiste um pacote do protocolo de foto
    def pacote_foto_correto(self, pct):
        compr_liquido = self.parse_be16(pct[4:6])
        if compr_liquido < 2:
            # Um pacote deveria ter no minimo um comando
            return False
        # Algoritmo de checksum tem propriedade interessante:
        # checksum de pacote sufixado com checksum resulta em 0
        return self.checksum(pct) == 0x00

    # Interpreta um pacote do protocolo de foto
    def pacote_foto_parse(self, pct):
        compr_liquido = self.parse_be16(pct[4:6])
        compr_payload = compr_liquido - 2
        cmd = self.parse_be16(pct[6:8])
        payload = pct[8:8+compr_payload]
        return cmd, payload

    def recv_callback(self, latest):
        self.log_debug("Conexao foto: recv", self.hexprint(latest))

        compr = self.pacote_foto_completo(self.recv_buf)
        if not compr:
            self.log_debug("Conexao foto: incompleto")
            return

        pct, self.recv_buf = self.recv_buf[:compr], self.recv_buf[compr:]

        if not self.pacote_foto_correto(pct):
            self.log_info("Conexao foto: pacote incorreto, desistindo")
            self.destroy()
            return

        cmd, payload = self.pacote_foto_parse(pct)
        self.log_debug("Conexao foto: resposta %04x" % cmd)

        if not self.tratador:
            self.log_info("Conexao foto: sem tratador")
            self.destroy()
            return

        self.conn_timeout.cancel()
        self.tratador(cmd, payload)
