package goalarmeitbl

import (
    "fmt"
    "log"
    "time"
    "slices"
    "github.com/ncruces/go-strftime"
)

type TratadorReceptorIP struct {
    receptor *ReceptorIP
    tcp *TCPSession
    buffer []byte
    central_identificada bool
    to_ident *Timeout
    to_comm *Timeout
    to_incompleta *Timeout
}

func NewTratadorReceptorIP(receptor *ReceptorIP, tcp *TCPSession) {
    t := new(TratadorReceptorIP)
    t.receptor = receptor
    t.tcp = tcp
    fmt.Println("TratadorReceptorIP: inicio")
    t.to_ident = t.tcp.Timeout(120 * time.Second, 0, "to_ident")
    t.to_comm = t.tcp.Timeout(600 * time.Second, 0, "to_comm")

    go func() {
        for evt := range t.tcp.Events {
            t.handle(evt)
        }
        fmt.Println("TratadorReceptorIP: fim ----")
    }()
}

func (t *TratadorReceptorIP) Bye() {
    t.tcp.Close()
}

// Handler dos eventos TCPClient/TCPSession
// Não-reentrante
func (t *TratadorReceptorIP) handle(evt Event) {
    switch evt.Name {
    case "Recv":
        buf, _ := evt.Cargo.([]byte)
        t.buffer = slices.Concat(t.buffer, buf)
        t.parse()
    case "Sent":
        // pass
    case "to_ident":
        fmt.Println("TratadorReceptorIP: timeout de identificação")
        t.Bye()
    case "to_comm":
        fmt.Println("TratadorReceptorIP: timeout de comunicação")
        t.Bye()
    case "to_incompleta":
        fmt.Println("TratadorReceptorIP: timeout de mensagem incompleta")
        t.Bye()
    case "SendEof", "RecvEof", "Err":
        fmt.Println("TratadorReceptorIP: Conexão terminada ", evt.Name)
        t.Bye()
    }
}

func (t *TratadorReceptorIP) msg_para_gancho(msg string) {
    msg = strftime.Format("%Y-%m-%dT%H:%M:%S", time.Now()) + " " + msg
    t.receptor.InvocaGancho("msg", msg)
}

func (t *TratadorReceptorIP) ev_para_gancho(codigo int, particao int, zona int, qualificador int) {
    msg := fmt.Sprintf("%d %d %d %d", codigo, particao, zona, qualificador)
    t.receptor.InvocaGancho("ev", msg)
}

func (t *TratadorReceptorIP) enviar(pacote PacoteRIP) {
    wiredata := pacote.Encode()
    log.Print("TratadorReceptorIP: Enviando ", HexPrint(wiredata))
    t.tcp.Send(wiredata)
}

func (t *TratadorReceptorIP) parse() {
    log.Print("TratadorReceptorIP: Recebido até agora ", HexPrint(t.buffer))
    t.to_comm.Restart()

    for {
        pacote, consumo := ExtrairFrameRIP(t.buffer)
        if consumo <= 0 {
            break
        }
        if t.to_incompleta != nil {
            t.to_incompleta.Free()
            t.to_incompleta = nil
        }
        t.buffer = t.buffer[consumo:]
        t.trata_pacote(pacote)
    }

    if len(t.buffer) > 0 && t.to_incompleta == nil {
        t.to_incompleta = t.tcp.Timeout(60 * time.Second, 0, "to_incompleta")
    }
}

func (t *TratadorReceptorIP) trata_pacote(pacote PacoteRIP) {
    if pacote.Longo {
        switch pacote.Tipo {
        case 0x00:
            // pacote invalidado pelo parser, por checksum inválido, etc.
            t.resposta_generica()
            break
        case 0x80:
            t.solicita_data_hora(pacote)
        case 0x94:
            t.identificacao_central(pacote)
        case 0xb0:
            t.evento_alarme(pacote, false)
        case 0xb5:
            t.evento_alarme(pacote, true)
        default:       
            fmt.Printf("TratadorReceptorIP: solicitação desconhecida %02x payload = %s\n", pacote.Tipo, HexPrint(pacote.Payload))
            t.resposta_generica()
        }
    } else {
        // pacote curto
        if pacote.Tipo == 0xf7 {
            log.Print("TratadorReceptorIP: heartbeat da central")
            t.resposta_generica()
        }
    }
}

func (t *TratadorReceptorIP) resposta_generica() {
    t.enviar(RIPRespostaGenerica())
}

func (t *TratadorReceptorIP) identificacao_central(pacote PacoteRIP) {
    defer t.resposta_generica()

    conta, macaddr, ok, msg := ParseRIPIdentificacaoCentral(pacote)

    if !ok {
        fmt.Println(msg)
        return
    }

    fmt.Printf("TratadorReceptorIP: identificacao central conta %d mac %s\n", conta, macaddr)

    // TODO? testar se central é autorizada a conectar, como na versão Python
    // TODO? testar número máximo conexões

    t.central_identificada = true
    if t.to_ident != nil {
        t.to_ident.Free()
        t.to_ident = nil
    }
}

func (t *TratadorReceptorIP) solicita_data_hora(pacote PacoteRIP) {
    fmt.Println("TratadorReceptorIP: solicitacao de data/hora pela central")
    t.enviar(RIPRespostaDataHora(time.Now()))
}

func (t *TratadorReceptorIP) evento_alarme(pacote PacoteRIP, com_foto bool) {
    defer t.resposta_generica()

    evento := ParseRIPAlarme(pacote, com_foto)
    if !evento.Valido {
        fmt.Println(evento.Erro)
        return
    }

    t.ev_para_gancho(evento.Codigo, evento.Particao, evento.Zona, evento.Qualificador)

    if evento.CodigoConhecido {
        fmt.Println(evento.DescricaoHumana)
        t.msg_para_gancho(evento.DescricaoHumana)
        // TODO? download fotos
    } else {
        msg := fmt.Sprintf("Evento de alarme canal %02x contact_id %d tipo %d qualificador %d " +
              "codigo %d particao %d zona %d", evento.Canal, evento.ContactId, evento.Tipo, evento.Qualificador,
              evento.Codigo, evento.Particao, evento.Zona)
        fmt.Println(msg)
        t.msg_para_gancho(msg)
    }
}
