package goalarmeitbl

import (
    "log"
    "time"
    "slices"
)

type TratadorResposta func (int, []byte)

type ObserverComando interface {
    Resultado(int)
}

type ComandoCentral struct {
    tcp *TCPClient
    observer ObserverComando
    timeout *Timeout
    senha int
    tam_senha int
    extra int
    buffer []byte

    tratador TratadorResposta

    status int
}

func NewComandoCentral(observer ObserverComando, serveraddr string, senha int, tam_senha int, extra int) *ComandoCentral {
    comando := new(ComandoCentral)
    comando.tcp = NewTCPClient(serveraddr, comando)
    comando.observer = observer
    comando.timeout = NewTimeout(15 * time.Second, 0, comando.tcp.Events, "Timeout")
    comando.senha = senha
    comando.tam_senha = tam_senha
    comando.extra = extra
    comando.status = 1 // erro
    log.Print("ComandoCentral: inicio")
    return comando
}

func (comando *ComandoCentral) Wait() {
    comando.tcp.Wait()
}

func (comando *ComandoCentral) Bye() {
    comando.observer.Resultado(comando.status)
    comando.timeout.Free()
    comando.tcp.Bye()
}

func (comando *ComandoCentral) Handle(_ *TCPClient, evt Event) bool {
    switch evt.Name {
    case "Connected":
        comando.Autenticar()
    case "NotConnected":
        log.Print("ComandoCentral: Conexão falhou")
        comando.Bye()
    case "Recv":
        comando.buffer = slices.Concat(comando.buffer, evt.Cargo)
        comando.Parse()
    case "Timeout":
        log.Print("ComandoCentral: Timeout")
        comando.Bye()
    case "SendEof", "RecvEof", "Err":
        log.Print("ComandoCentral: Conexão terminada ", evt.Name)
        comando.Bye()
    default:
        return false
    }
    return true
}

func (comando *ComandoCentral) EnviarPacote(pacote []byte, tf TratadorResposta) {
    log.Print("ComandoCentral: Enviando ", HexPrint(pacote))
    // reinicia timer de desconexão
    comando.timeout.Restart()
    // configura tratador da resposta
    comando.tratador = tf
    comando.tcp.Send(pacote)
}

func (comando *ComandoCentral) Autenticar() {
    log.Print("ComandoCentral: Autenticando")
    pacote := PacoteIsecNet2Auth(comando.senha, comando.tam_senha)
    comando.EnviarPacote(pacote, comando.RespostaAutenticacao)
}

func (comando *ComandoCentral) Parse() {
    log.Print("ComandoCentral: Recebido até agora ", HexPrint(comando.buffer))
    comprimento := PacoteIsecNet2Completo(comando.buffer)
    if comprimento == 0 {
        log.Print("ComandoCentral: Pacote incompleto")
        return
    }
    
    pacote := comando.buffer[:comprimento]
    comando.buffer = comando.buffer[comprimento:]

    if !PacoteIsecNet2Correto(pacote) {
        log.Print("ComandoCentral: Pacote incorreto, desistindo")
        comando.Bye()
    }

    cmd, payload := PacoteIsecNet2Parse(pacote)
    log.Printf("ComandoCentral: Pacote resposta %04x", cmd)

    if comando.tratador == nil {
        log.Printf("ComandoCentral: sem tratador")
        comando.Bye()
    }
    comando.tratador(cmd, payload)
}

func (comando *ComandoCentral) RespostaAutenticacao(cmd int, payload []byte) {
    if cmd == 0xf0fd {
        comando.ParseNak(payload)
        comando.Bye()
        return
    }

    if cmd != 0xf0f0 {
        log.Printf("ComandoCentral: auth resp inesperada %04x", cmd)
        comando.Bye()
        return
    }

    if len(payload) != 1 {
        log.Printf("ComandoCentral: auth resp invalida")
        comando.Bye()
        return
    }

    resposta := int(payload[0])
    // Possíveis respostas:
    // 01 = senha incorreta
    // 02 = versão software incorreta
    // 03 = painel chamará de volta (?)
    // 04 = aguardando permissão de usuário (?)

    if resposta > 0 {
        log.Printf("ComandoCentral: auth falhou por motivo %d", resposta)
        comando.Bye()
        return
    }

    log.Printf("ComandoCentral: auth ok")
    // TODO invocar comando subordinado
    comando.Despedir()
}

func (comando *ComandoCentral) ParseNak(payload []byte) {
    if len(payload) != 1 {
        log.Printf("ComandoCentral: nak invalido")
        return
    }
    log.Printf("ComandoCentral: nak motivo %02x", int(payload[0]))
}

func (comando *ComandoCentral) Despedir() {
    log.Print("ComandoCentral: Despedindo")
    // envia pacote de despedida à central
    pacote := PacoteIsecNet2Bye()
    comando.EnviarPacote(pacote, nil)
    // fecha conexão no sentido tx
    comando.tcp.Send(nil)
    // reportar sucesso para camadas superiores, ao encerrar
    comando.status = 0
}
