package goalarmeitbl

import (
    "log"
    "fmt"
    "time"
    "slices"
)

// Função que vai tratar a resposta vinda da central
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

// Comando à central.
// Esta estrutura implementa apenas a infra-estrutura para um comando (conexão e autenticação)
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
    wait chan struct{}
}

// Cria novo comando e inicia a conexão à central
// Usuário deve chamar Wait() e receber o resultado através do callback ao observador
func NewComandoCentral(sub ComandoCentralSub, observer ObserverComando, serveraddr string, senha int, tam_senha int) *ComandoCentral {
    comando := new(ComandoCentral)
    comando.tcp = NewTCPClient(serveraddr)
    comando.sub = sub
    comando.observer = observer
    comando.timeout = comando.tcp.Timeout(15 * time.Second, 0, "Timeout")
    comando.senha = senha
    comando.tam_senha = tam_senha
    comando.status = 1 // erro
    comando.wait = make(chan struct{}, 1)
    log.Print("ComandoCentral: inicio")

    go func() {
        for evt := range comando.tcp.Events {
            switch evt.Name {
            case "Connected":
                comando.autenticar()
            case "NotConnected":
                fmt.Println("ComandoCentral: Conexão falhou")
                comando.Bye()
            case "Recv":
                buf, _ := evt.Cargo.([]byte)
                comando.buffer = slices.Concat(comando.buffer, buf)
                comando.parse()
            case "Timeout":
                fmt.Println("ComandoCentral: Timeout")
                comando.Bye()
            case "SendEof", "RecvEof", "Err":
                log.Print("ComandoCentral: Conexão terminada ", evt.Name)
                comando.Bye()
            }
        }
        comando.wait <-struct{}{}
        log.Print("ComandoCentral: fim ----")
    }()

    return comando
}

// Envia pacote de autenticação
func (comando *ComandoCentral) autenticar() {
    log.Print("ComandoCentral: Autenticando")
    pacote := PacoteIsecNet2Auth(comando.senha, comando.tam_senha)
    comando.EnviarPacote(pacote, comando.resposta_autenticacao)
}

// Interpreta um pacote de resposta da central
func (comando *ComandoCentral) parse() {
    log.Print("ComandoCentral: Recebido até agora ", HexPrint(comando.buffer))
    comprimento := PacoteIsecNet2Completo(comando.buffer)
    if comprimento == 0 {
        log.Print("ComandoCentral: Pacote incompleto")
        return
    }

    pacote := comando.buffer[:comprimento]
    comando.buffer = comando.buffer[comprimento:]

    if !PacoteIsecNet2Correto(pacote) {
        fmt.Println("ComandoCentral: Pacote incorreto, desistindo")
        comando.Bye()
    }

    cmd, payload := PacoteIsecNet2Parse(pacote)
    log.Printf("ComandoCentral: Pacote resposta %04x", cmd)

    if cmd == 0xf0fd {
        comando.parse_nak(payload)
        comando.Bye()
        return
    } else if cmd == 0xf0f7 {
        fmt.Println("ComandoCentral: central ocupada")
        comando.Bye()
        return
    }

    if comando.tratador_resposta == nil {
        log.Print("ComandoCentral: sem tratador")
        comando.Bye()
    }
    comando.tratador_resposta(comando, cmd, payload)
}

func (comando *ComandoCentral) resposta_autenticacao(_ *ComandoCentral, cmd int, payload []byte) {
    if cmd != 0xf0f0 {
        fmt.Printf("ComandoCentral: auth resp inesperada %04x\n", cmd)
        comando.Bye()
        return
    }

    if len(payload) != 1 {
        fmt.Println("ComandoCentral: auth resp invalida")
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
        fmt.Printf("ComandoCentral: auth falhou por motivo %d\n", resposta)
        comando.Bye()
        return
    }

    log.Print("ComandoCentral: auth ok")
    // Delega a comunicação a ComandoCentralSub
    comando.sub.Autenticado(comando)
}

// Interpreta pacote "NAK" de erro
func (comando *ComandoCentral) parse_nak(payload []byte) {
    if len(payload) != 1 {
        fmt.Println("ComandoCentral: nak invalido")
        return
    }
    fmt.Printf("ComandoCentral: nak motivo %02x\n", int(payload[0]))
}

// Envia pacote de comando e implanta um tratador da resposta
// Invocado tanto aqui como pela subclasse
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

// Encerra a comunicação com a central de forma "civilizada"
// Invocado pela subclasse
func (comando *ComandoCentral) Despedida() {
    log.Print("ComandoCentral: Despedindo")
    pacote := PacoteIsecNet2Bye()
    comando.EnviarPacote(pacote, nil)
    // reportar sucesso para camadas superiores, ao encerrar
    comando.status = 0
}

// Aborta o comando
// Invocado tanto aqui como pela subclasse
func (comando *ComandoCentral) Bye() {
    comando.observer.Resultado(comando.status)
    comando.tcp.Close()
}

// Bloqueia até o comando ser concluído
// Invocado pelo usuário
func (comando *ComandoCentral) Wait() {
    <-comando.wait
}
