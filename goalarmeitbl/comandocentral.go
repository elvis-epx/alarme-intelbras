package goalarmeitbl

import (
    "log"
    "time"
    "slices"
    "sync"
)

// Função que vai tratar a resposta  vinda da central
type TratadorResposta func (*ComandoCentral, int, []byte)

// Observador do resultado final do comando
type ObserverComando interface {
    // invocado quando conexão termina
    Resultado(int)
}

// Implementação ("subclasse") do comando à central
type ComandoCentralSub interface {
    Autenticado(*ComandoCentral)
}

type ComandoCentral struct {
    tcp *TCPClient
    sub ComandoCentralSub
    observer ObserverComando
    timeout *Timeout
    senha int
    tam_senha int
    buffer []byte
    tratador_resposta TratadorResposta
    status int
    wg sync.WaitGroup
}

func NewComandoCentral(sub ComandoCentralSub, observer ObserverComando, serveraddr string, senha int, tam_senha int) *ComandoCentral {
    comando := new(ComandoCentral)
    comando.tcp = NewTCPClient(serveraddr)
    comando.sub = sub
    comando.observer = observer
    comando.timeout = NewTimeout(15 * time.Second, 0, comando.tcp.Events, "Timeout")
    comando.senha = senha
    comando.tam_senha = tam_senha
    comando.status = 1 // erro
    comando.wg = sync.WaitGroup{}
    comando.wg.Add(1)
    log.Print("ComandoCentral: inicio")

    go func() {
        for evt := range comando.tcp.Events {
            comando.handle(evt)
        }
        comando.wg.Done()
        log.Print("ComandoCentral: fim ----")
    }()

    return comando
}

func (comando *ComandoCentral) Bye() {
    comando.observer.Resultado(comando.status)
    comando.timeout.Free()
    comando.tcp.Close()
}

func (comando *ComandoCentral) Wait() {
    comando.wg.Wait()
}

func (comando *ComandoCentral) handle(evt Event) {
    switch evt.Name {
    case "Connected":
        comando.Autenticar()
    case "NotConnected":
        log.Print("ComandoCentral: Conexão falhou")
        comando.Bye()
    case "Recv":
        buf, _ := evt.Cargo.([]byte)
        comando.buffer = slices.Concat(comando.buffer, buf)
        comando.Parse()
    case "Timeout":
        log.Print("ComandoCentral: Timeout")
        comando.Bye()
    case "SendEof", "RecvEof", "Err":
        log.Print("ComandoCentral: Conexão terminada ", evt.Name)
        comando.Bye()
    }
}

func (comando *ComandoCentral) EnviarPacote(pacote []byte, tf TratadorResposta) {
    log.Print("ComandoCentral: Enviando ", HexPrint(pacote))
    comando.timeout.Restart()
    comando.tratador_resposta = tf
    comando.tcp.Send(pacote)
    if tf == nil {
        // fecha conexão no sentido tx
        comando.tcp.Send(nil)
    }
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

    if cmd == 0xf0fd {
        comando.ParseNak(payload)
        comando.Bye()
        return
    } else if cmd == 0xf0f7 {
        log.Printf("ComandoCentral: central ocupada")
        comando.Bye()
        return
    }

    if comando.tratador_resposta == nil {
        log.Print("ComandoCentral: sem tratador")
        comando.Bye()
    }
    comando.tratador_resposta(comando, cmd, payload)
}

func (comando *ComandoCentral) RespostaAutenticacao(_ *ComandoCentral, cmd int, payload []byte) {
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

    log.Print("ComandoCentral: auth ok")
    comando.sub.Autenticado(comando)
}

func (comando *ComandoCentral) ParseNak(payload []byte) {
    if len(payload) != 1 {
        log.Print("ComandoCentral: nak invalido")
        return
    }
    log.Printf("ComandoCentral: nak motivo %02x", int(payload[0]))
}

func (comando *ComandoCentral) Despedida() {
    log.Print("ComandoCentral: Despedindo")
    pacote := PacoteIsecNet2Bye()
    comando.EnviarPacote(pacote, nil)
    // reportar sucesso para camadas superiores, ao encerrar
    comando.status = 0
}
