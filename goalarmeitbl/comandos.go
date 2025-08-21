package goalarmeitbl

import (
    "log"
    "slices"
    "strings"
    "fmt"
)

// Descritor de uma subclasse, para usar num mapa string -> descritor
type DescComandoSub struct {
    ExtraHelp string
    ExtraParam bool
    Construtor Constructor
}

// ComandoNulo (no-op)

type ComandoNulo struct {
}

func (comando *ComandoNulo) Autenticado(super *ComandoCentral) {
    super.Despedida()
}

func NewComandoNulo(_ int) ComandoCentralSub {
    comando := new(ComandoNulo)
    return comando
}

// SolicitarStatus

type SolicitarStatus struct {
}

func (comando *SolicitarStatus) Autenticado(super *ComandoCentral) {
    pacote := PacoteIsecNet2(0x0b4a, nil)
    super.EnviarPacote(pacote, comando.RespostaStatus)
}

func bits_para_numeros(octetos []byte, inverso bool) string {
    lista := []string{}
    for i, octeto := range octetos {
        for j := range 8 {
            bit := (octeto & (1 << j))
            if (bit != 0 && !inverso) || (bit == 0 && inverso) {
                lista = append(lista, fmt.Sprintf("%d", (1 + j + i * 8)))
            }
        }
    }
    return strings.Join(lista, ", ")
}

func sim_nao(valor int) string {
    if valor != 0 {
        return "Sim"
    }
    return "Não"
}

func (comando *SolicitarStatus) RespostaStatus(super *ComandoCentral, cmd int, payload []byte) {
    payload = slices.Concat([]byte{0x00}, payload)
    log.Print()
    log.Print()
    log.Print("*******************************************")
    if payload[1] == 0x01 {
        log.Print("Central AMT-8000")
    } else {
        log.Print("Central de tipo desconhecido")
    }
    log.Printf("Versão de firmware %d.%d.%d", payload[2], payload[3], payload[4])
    log.Print("Status geral: ")
    var armado = map[int]string{0x00: "Desarmado", 0x01: "Partição(ões) armada(s)", 0x03: "Todas partições armadas"}
    log.Printf("\t %s", armado[int(((payload[21] >> 5) & 0x03))])
    log.Printf("\tZonas em alarme: %s", sim_nao(int(payload[21] & 0x8)))
    log.Printf("\tZonas canceladas: %s", sim_nao(int(payload[21] & 0x10)))
    log.Printf("\tTodas zonas fechadas: %s", sim_nao(int(payload[21] & 0x4)))
    log.Printf("\tSirene: %s", sim_nao(int(payload[21] & 0x2)))
    log.Printf("\tProblemas: %s", sim_nao(int(payload[21] & 0x1)))
    for particao := range 17 {
        habilitado := payload[22 + particao] & 0x80
        if habilitado == 0 {
            continue
        }
        log.Printf("Partição %02d:", particao)
        log.Printf("\tStay: %s", sim_nao(int(payload[22 + particao] & 0x40)))
        log.Printf("\tDelay de saída: %s", sim_nao(int(payload[22 + particao] & 0x20)))
        log.Printf("\tPronto para armar: %s", sim_nao(int(payload[22 + particao] & 0x10)))
        log.Printf("\tAlame ocorreu: %s", sim_nao(int(payload[22 + particao] & 0x08)))
        log.Printf("\tEm alarme: %s", sim_nao(int(payload[22 + particao] & 0x04)))
        log.Printf("\tArmado modo stay: %s", sim_nao(int(payload[22 + particao] & 0x02)))
        log.Printf("\tArmado: %s", sim_nao(int(payload[22 + particao] & 0x01)))
    }
    log.Printf("Zonas abertas: %s", bits_para_numeros(payload[39:47], false))
    log.Printf("Zonas em alarme: %s", bits_para_numeros(payload[47:55], false))
    // log.Printf("Zonas ativas: %s", bits_para_numeros(payload[55:63], true))
    log.Printf("Zonas em bypass: %s", bits_para_numeros(payload[55:63], false))
    log.Printf("Sirenes ligadas: %s", bits_para_numeros(payload[63:65], false))

    // TODO interpretar mais campos
    log.Print("*******************************************")
    log.Print()

    super.Despedida()
}

func NewSolicitarStatus(_ int) ComandoCentralSub {
    comando := new(SolicitarStatus)
    return comando
}

// DesativarCentral

type DesativarCentral struct {
    particao int
}

func (comando *DesativarCentral) Autenticado(super *ComandoCentral) {
    // byte 1: particao (0x01 = 1, 0xff = todas ou sem particao)
    // byte 2: 0x00 desarmar, 0x01 armar, 0x02 stay
    pacote := PacoteIsecNet2(0x401e, []byte{byte(comando.particao), 0x00})
    super.EnviarPacote(pacote, comando.RespostaDesativarCentral)
}

func (comando *DesativarCentral) RespostaDesativarCentral(super *ComandoCentral, cmd int, payload []byte) {
    super.Despedida()
}

func NewDesativarCentral(particao int) ComandoCentralSub {
    comando := new(DesativarCentral)
    if particao == 0 {
        // todas as partições
        comando.particao = 0xff
    } else {
        comando.particao = particao
    }
    return comando
}

// AtivarCentral

type AtivarCentral struct {
    particao int
}

func (comando *AtivarCentral) Autenticado(super *ComandoCentral) {
    // byte 1: particao (0x01 = 1, 0xff = todas ou sem particao)
    // byte 2: 0x00 desarmar, 0x01 armar, 0x02 stay
    pacote := PacoteIsecNet2(0x401e, []byte{byte(comando.particao), 0x01})
    super.EnviarPacote(pacote, comando.RespostaAtivarCentral)
}

func (comando *AtivarCentral) RespostaAtivarCentral(super *ComandoCentral, cmd int, payload []byte) {
    super.Despedida()
}

func NewAtivarCentral(particao int) ComandoCentralSub {
    comando := new(AtivarCentral)
    if particao == 0 {
        // todas as partições
        comando.particao = 0xff
    } else {
        comando.particao = particao
    }
    return comando
}

// DesligarSirene

type DesligarSirene struct {
    particao byte
}

func (comando *DesligarSirene) Autenticado(super *ComandoCentral) {
    pacote := PacoteIsecNet2(0x4019, []byte{comando.particao})
    super.EnviarPacote(pacote, comando.RespostaDesligarSirene)
}

func (comando *DesligarSirene) RespostaDesligarSirene(super *ComandoCentral, cmd int, payload []byte) {
    super.Despedida()
}

func NewDesligarSirene(particao int) ComandoCentralSub {
    comando := new(DesligarSirene)
    if particao == 0 {
        // todas as partições
        comando.particao = 0xff
    } else {
        comando.particao = byte(particao)
    }
    return comando
}

// BypassZona

type BypassZona struct {
    zona byte
}

func (comando *BypassZona) Autenticado(super *ComandoCentral) {
    pacote := PacoteIsecNet2(0x401f, []byte{comando.zona - 1, 0x01})
    super.EnviarPacote(pacote, comando.RespostaBypassZona)
}

func (comando *BypassZona) RespostaBypassZona(super *ComandoCentral, cmd int, payload []byte) {
    super.Despedida()
}

func NewBypassZona(zona int) ComandoCentralSub {
    comando := new(BypassZona)
    if zona < 1 || zona > 254 {
        log.Fatal("Zona precisa ser especificada")
    }
    comando.zona = byte(zona)
    return comando
}

// ReativarZona

type ReativarZona struct {
    zona byte
}

func (comando *ReativarZona) Autenticado(super *ComandoCentral) {
    pacote := PacoteIsecNet2(0x401f, []byte{comando.zona - 1, 0x00})
    super.EnviarPacote(pacote, comando.RespostaReativarZona)
}

func (comando *ReativarZona) RespostaReativarZona(super *ComandoCentral, cmd int, payload []byte) {
    super.Despedida()
}

func NewReativarZona(zona int) ComandoCentralSub {
    comando := new(ReativarZona)
    if zona < 1 || zona > 254 {
        log.Fatal("Zona precisa ser especificada")
    }
    comando.zona = byte(zona)
    return comando
}

// LimparDisparo

type LimparDisparo struct {
}

func (comando *LimparDisparo) Autenticado(super *ComandoCentral) {
    pacote := PacoteIsecNet2(0x4013, nil)
    super.EnviarPacote(pacote, comando.RespostaLimparDisparo)
}

func (comando *LimparDisparo) RespostaLimparDisparo(super *ComandoCentral, cmd int, payload []byte) {
    super.Despedida()
}

func NewLimparDisparo(_ int) ComandoCentralSub {
    comando := new(LimparDisparo)
    return comando
}

// Lista de comandos disponíveis

var Subcomandos map[string]DescComandoSub

func init() {
    Subcomandos = map[string]DescComandoSub{
        "nulo": DescComandoSub{"", false, NewComandoNulo},
        "status": DescComandoSub{"", false, NewSolicitarStatus},
        "ativar": DescComandoSub{"[partição] ou todas partições se omitida", true, NewAtivarCentral},
        "desativar": DescComandoSub{"[partição] ou todas partições se omitida", true, NewDesativarCentral},
        "desligarsirene": DescComandoSub{"[partição] ou todas partições se omitida", true, NewDesligarSirene},
        "limpardisparo": DescComandoSub{"", false, NewLimparDisparo},
        "bypass": DescComandoSub{"<zona 1-64>", true, NewBypassZona},
        "reativar": DescComandoSub{"<zona 1-64>", true, NewReativarZona},
    }
}
