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
    send_queue [][]byte
    send_inflight int
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
    t.to_ident = t.Timeout(120 * time.Second, 0, t.tcp.Events, "to_ident")
    t.to_comm = t.Timeout(600 * time.Second, 0, t.tcp.Events, "to_comm")

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
        log.Print("TratadorReceptorIP: Sent")
        t.send_inflight -= 1
        t._send()
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

func (t *TratadorReceptorIP) _send() {
    if len(t.send_queue) == 0 || t.send_inflight > 0 {
        return
    }

    // como tcp.Send() não bloqueia, e falha se a fila estiver cheia, escolhemos enviar apenas uma
    // resposta de cada vez usando o evento "Sent" para coordenar
    
    log.Print("TratadorReceptorIP: Sending")
    if !t.tcp.Send(t.send_queue[0]) {
        log.Fatal("tcp.Send() não deveria falhar")
    }
    t.send_queue = t.send_queue[1:]
    t.send_inflight += 1
}

func (t *TratadorReceptorIP) enviar(pacote []byte) {
    log.Print("TratadorReceptorIP: Enviando ", HexPrint(pacote))
    t.send_queue = append(t.send_queue, pacote)
    t._send()
}

func (t *TratadorReceptorIP) enquadrar(dados []byte) []byte {
    pacote := slices.Concat([]byte{byte(len(dados))}, dados)
    pacote = append(pacote, Checksum(pacote))
    return pacote
}

func (t *TratadorReceptorIP) enviar_longo(dados []byte) {
    t.enviar(t.enquadrar(dados))
}

func (t *TratadorReceptorIP) enviar_curto(dados []byte) {
    t.enviar(dados)
}

func (t *TratadorReceptorIP) parse() {
    log.Print("TratadorReceptorIP: Recebido até agora ", HexPrint(t.buffer))
    t.to_comm.Restart()

    for t.consome_msg() {
        // pass
    }
}

func (t *TratadorReceptorIP) consome_msg() bool {
    if t.consome_frame_curto() || t.consome_frame_longo() {
        if t.to_incompleta != nil {
            t.to_incompleta.Free()
            t.to_incompleta = nil
        }
        return true
    }

    if len(t.buffer) > 0 {
        if t.to_incompleta == nil {
            t.to_incompleta = t.Timeout(60 * time.Second, 0, t.tcp.Events, "to_incompleta")
        }
    }

    return false
}

func (t *TratadorReceptorIP) consome_frame_curto() bool {
    if len(t.buffer) > 0 && t.buffer[0] == 0xf7 {
        t.buffer = t.buffer[1:]
        log.Print("TratadorReceptorIP: heartbeat da central")
        t.resposta_generica()
        return true
    }
    return false
}

func (t *TratadorReceptorIP) consome_frame_longo() bool {
    if len(t.buffer) < 2 {
        return false
    }

    esperado := int(t.buffer[0]) + 2 // comprimento + dados + checksum
    if len(t.buffer) < esperado {
        return false
    }

    rawmsg := t.buffer[:esperado]
    t.buffer = t.buffer[esperado:]

    // checksum de pacote sufixado com checksum resulta em 0
    if Checksum(rawmsg) != 0x00 {
        fmt.Println("TratadorReceptorIP: checksum errado, rawmsg =", HexPrint(rawmsg))
        return true
    }

    // Mantém checksum no final pois, em algumas mensagens, o último octeto
    // calcula como checksum mas tem outro significado (e.g. 0xb5)
    msg := rawmsg[1:]

    if len(msg) == 0 {
        fmt.Println("TratadorReceptorIP: mensagem nula")
        return true
    }

    tipo := int(msg[0])
    msg = msg[1:]

    switch tipo {
    case 0x80:
        t.solicita_data_hora(msg)
    case 0x94:
        t.identificacao_central(msg)
    case 0xb0:
        t.evento_alarme(msg, false)
    case 0xb5:
        t.evento_alarme(msg, true)
    default:       
        fmt.Printf("TratadorReceptorIP: solicitação desconhecida %02x payload = %s\n", tipo, HexPrint(msg))
        t.resposta_generica()
    }

    return true
}

func (t *TratadorReceptorIP) resposta_generica() {
    resposta := []byte{0xfe}
    t.enviar_curto(resposta)
}

func (t *TratadorReceptorIP) identificacao_central(msg []byte) {
    resposta := []byte{0xfe}

    if len(msg) != 7 {
        fmt.Printf("TratadorReceptorIP: tamanho inesperado %s\n", HexPrint(msg))
        t.enviar_curto(resposta)
    }

    // canal := msg[0] // 'E' (0x45)=Ethernet, 'G'=GPRS, 'H'=GPRS2
    conta, _ := FromBCD(msg[1:3])
    macaddr := HexPrint(msg[3:6])
    fmt.Printf("TratadorReceptorIP: identificacao central conta %d mac %s\n", conta, macaddr)

    // TODO? testar se central é autorizada a conectar, como na versão Python
    // TODO? testar número máximo conexões

    t.central_identificada = true
    if t.to_ident != nil {
        t.to_ident.Free()
        t.to_ident = nil
    }

    t.enviar_curto(resposta)
}

func (t *TratadorReceptorIP) solicita_data_hora(msg []byte) {
    fmt.Println("TratadorReceptorIP: solicitacao de data/hora pela central")
    agora := time.Now()

    year := agora.Year()
	month := int(agora.Month())
	day := agora.Day()
	hour := agora.Hour()
	minute := agora.Minute()
	second := agora.Second()
    // em Go, time.Weekday() retorna 0 para domingo
    // e o protocolo da central adota a mesma convenção
    dow := int(agora.Weekday())
    fmt.Printf("TratadorReceptorIP: %04d-%02d-%02d %02d:%02d\n", year, month, day, hour, minute, second)

    resposta := []byte{0x80, BCD(year - 2000), BCD(month), BCD(day), BCD(dow), BCD(hour), BCD(minute), BCD(second)}
    t.enviar_longo(resposta)
}

func (t *TratadorReceptorIP) evento_alarme(msg []byte, com_foto bool) {
    compr := 17
    if com_foto {
        compr = 20
    }

    if len(msg) != compr {
        fmt.Println("TratadorReceptorIP: evento de alarme tamanho inesperado ", HexPrint(msg))
        t.resposta_generica()
        return
    }

    canal := int(msg[0]) // 0x11 Ethernet IP1, 0x12 IP2, 0x21 GPRS IP1, 0x22 IP2
    contact_id, err := ContactIDDecode(msg[1:5])
    if err != nil {
        fmt.Println("TratadorReceptorIP: contact_id inválido")
        t.resposta_generica()
        return
    }
    tipo_msg, err := ContactIDDecode(msg[5:7]) // 18 decimal = Contact ID
    if err != nil {
        fmt.Println("TratadorReceptorIP: tipo_msg inválido")
        t.resposta_generica()
        return
    }
    qualificador := int(msg[7])
    codigo, err := ContactIDDecode(msg[8:11])
    if err != nil {
        fmt.Println("TratadorReceptorIP: qualificador inválido")
        t.resposta_generica()
        return
    }
    particao, err := ContactIDDecode(msg[11:13])
    if err != nil {
        fmt.Println("TratadorReceptorIP: partição inválida")
        t.resposta_generica()
        return
    }
    zona, err := ContactIDDecode(msg[13:16])
    if err != nil {
        fmt.Println("TratadorReceptorIP: zona inválida")
        t.resposta_generica()
        return
    }
    indice := 0
    nr_fotos := 0
    if com_foto {
        // checksum := msg[16] // truque do protocolo de reposicionar o checksum
        indice = int(msg[17]) * 256 + int(msg[18])
        nr_fotos = int(msg[19])
    }

    t.ev_para_gancho(codigo, particao, zona, qualificador)

    evento_contact_id, eok := EventosContactID[codigo]
    squalif := ""
    desconhecido := true

    if tipo_msg == 18 && eok {
        if qualificador == 1 {
            squalif = "aber"
            _, sok := evento_contact_id[squalif]
            if !sok {
                squalif = "*"
            }
        } else if qualificador == 3 {
            squalif = "rest"
            _, sok := evento_contact_id[squalif]
            if !sok {
                squalif = "*"
            }
        } else {
            squalif = "*"
        }

        scodigo, sok := evento_contact_id[squalif]

        if sok {
            desconhecido = false
            descricao_humana := fmt.Sprintf(scodigo, zona, particao)
            if com_foto {
                fotos := fmt.Sprintf("(com fotos, i=%d n=%d)", indice, nr_fotos)
                descricao_humana += fotos
            }
            fmt.Println(descricao_humana)
            t.msg_para_gancho(descricao_humana)
            // TODO? download fotos
        }
    }

    if desconhecido {
        msg := fmt.Sprintf("Evento de alarme canal %02x contact_id %d tipo %d qualificador %d " +
              "codigo %d particao %d zona %d", canal, contact_id, tipo_msg, qualificador, codigo, particao, zona)
        fmt.Println(msg)
        t.msg_para_gancho(msg)
    }

    t.resposta_generica()
}
