package goalarmeitbl

import (
    "slices"
    "strings"
    "fmt"
)

// Construtor de uma implementação/subclasse
type Constructor func(int) (ComandoCentralSub, string)

// Descritor de uma subclasse, para usar num mapa string -> descritor
type DescComandoSub struct {
    ExtraHelp string
    ExtraParam bool
    Construtor Constructor
}

// Apenas autentica e encerra.
// Útil para testes, conferir que a senha é válida, etc.
type ComandoNulo struct {
}

func (comando *ComandoNulo) Autenticado(super *ComandoCentral) {
    super.Despedida()
}

func NewComandoNulo(_ int) (ComandoCentralSub, string) {
    comando := new(ComandoNulo)
    return comando, ""
}

// Solicita status da central: partições, disparos, etc.
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
    if cmd != 0x0b4a {
        fmt.Printf("RespostaStatus: resp inesperada %04x\n", cmd)
        super.Bye()
        return
    }

    payload = slices.Concat([]byte{0x00}, payload)
    fmt.Println()
    fmt.Println()
    fmt.Println("*******************************************")
    if payload[1] == 0x01 {
        fmt.Println("Central AMT-8000")
    } else {
        fmt.Println("Central de tipo desconhecido")
    }
    fmt.Printf("Versão de firmware %d.%d.%d\n", payload[2], payload[3], payload[4])
    fmt.Println("Status geral: ")
    var armado = map[int]string{0x00: "Desarmado", 0x01: "Partição(ões) armada(s)", 0x03: "Todas partições armadas"}
    fmt.Printf("\t %s\n", armado[int(((payload[21] >> 5) & 0x03))])
    fmt.Printf("\tZonas em alarme: %s\n", sim_nao(int(payload[21] & 0x8)))
    fmt.Printf("\tZonas canceladas: %s\n", sim_nao(int(payload[21] & 0x10)))
    fmt.Printf("\tTodas zonas fechadas: %s\n", sim_nao(int(payload[21] & 0x4)))
    fmt.Printf("\tSirene: %s\n", sim_nao(int(payload[21] & 0x2)))
    fmt.Printf("\tProblemas: %s\n", sim_nao(int(payload[21] & 0x1)))
    for particao := range 17 {
        habilitado := payload[22 + particao] & 0x80
        if habilitado == 0 {
            continue
        }
        fmt.Printf("Partição %02d:\n", particao)
        fmt.Printf("\tStay: %s\n", sim_nao(int(payload[22 + particao] & 0x40)))
        fmt.Printf("\tDelay de saída: %s\n", sim_nao(int(payload[22 + particao] & 0x20)))
        fmt.Printf("\tPronto para armar: %s\n", sim_nao(int(payload[22 + particao] & 0x10)))
        fmt.Printf("\tAlame ocorreu: %s\n", sim_nao(int(payload[22 + particao] & 0x08)))
        fmt.Printf("\tEm alarme: %s\n", sim_nao(int(payload[22 + particao] & 0x04)))
        fmt.Printf("\tArmado modo stay: %s\n", sim_nao(int(payload[22 + particao] & 0x02)))
        fmt.Printf("\tArmado: %s\n", sim_nao(int(payload[22 + particao] & 0x01)))
    }
    fmt.Printf("Zonas abertas: %s\n", bits_para_numeros(payload[39:47], false))
    fmt.Printf("Zonas em alarme: %s\n", bits_para_numeros(payload[47:55], false))
    // fmt.Printf("Zonas ativas: %s\n", bits_para_numeros(payload[55:63], true))
    fmt.Printf("Zonas em bypass: %s\n", bits_para_numeros(payload[55:63], false))
    fmt.Printf("Sirenes ligadas: %s\n", bits_para_numeros(payload[63:65], false))

    // TODO interpretar mais campos
    fmt.Println("*******************************************")
    fmt.Println()

    super.Despedida()
}

func NewSolicitarStatus(_ int) (ComandoCentralSub, string) {
    comando := new(SolicitarStatus)
    return comando, ""
}

// Desarmar o alarme da central
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
    if cmd != 0x401e {
        fmt.Printf("DesativarCentral: resp inesperada %04x\n", cmd)
        super.Bye()
        return
    }

    super.Despedida()
}

func NewDesativarCentral(particao int) (ComandoCentralSub, string) {
    comando := new(DesativarCentral)
    if particao == 0 {
        // todas as partições
        comando.particao = 0xff
    } else {
        comando.particao = particao
    }
    return comando, ""
}

// Ativar o alarme, ou seja, armar a central
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
    if cmd != 0x401e {
        fmt.Printf("AtivarCentral: resp inesperada %04x\n", cmd)
        super.Bye()
        return
    }

    super.Despedida()
}

func NewAtivarCentral(particao int) (ComandoCentralSub, string) {
    comando := new(AtivarCentral)
    if particao == 0 {
        // todas as partições
        comando.particao = 0xff
    } else {
        comando.particao = particao
    }
    return comando, ""
}

// Desligar a sirene, sem limpar o disparo do alarme em si
type DesligarSirene struct {
    particao byte
}

func (comando *DesligarSirene) Autenticado(super *ComandoCentral) {
    pacote := PacoteIsecNet2(0x4019, []byte{comando.particao})
    super.EnviarPacote(pacote, comando.RespostaDesligarSirene)
}

func (comando *DesligarSirene) RespostaDesligarSirene(super *ComandoCentral, cmd int, payload []byte) {
    if cmd != 0xf0fe {
        fmt.Printf("DesligarSirene: resp inesperada %04x\n", cmd)
        super.Bye()
        return
    }

    super.Despedida()
}

func NewDesligarSirene(particao int) (ComandoCentralSub, string) {
    comando := new(DesligarSirene)
    if particao == 0 {
        // todas as partições
        comando.particao = 0xff
    } else {
        comando.particao = byte(particao)
    }
    return comando, ""
}

// Bypass de zona, ou seja, desativar a zona de modo que não possa disparar o alarme
type BypassZona struct {
    zona byte
}

func (comando *BypassZona) Autenticado(super *ComandoCentral) {
    pacote := PacoteIsecNet2(0x401f, []byte{comando.zona - 1, 0x01})
    super.EnviarPacote(pacote, comando.RespostaBypassZona)
}

func (comando *BypassZona) RespostaBypassZona(super *ComandoCentral, cmd int, payload []byte) {
    if cmd != 0xf0fe {
        fmt.Printf("BypassZona: resp inesperada %04x\n", cmd)
        super.Bye()
        return
    }

    super.Despedida()
}

func NewBypassZona(zona int) (ComandoCentralSub, string) {
    comando := new(BypassZona)
    if zona < 1 || zona > 254 {
        return nil, "Zona precisa ser especificada e estar na faixa 1-254"
    }
    comando.zona = byte(zona)
    return comando, ""
}

// Reativar zona, ou seja, remover o bypass de zona
type ReativarZona struct {
    zona byte
}

func (comando *ReativarZona) Autenticado(super *ComandoCentral) {
    pacote := PacoteIsecNet2(0x401f, []byte{comando.zona - 1, 0x00})
    // TODO implementar reativação todas as zonas (0xff + códigos x 0x3f)
    super.EnviarPacote(pacote, comando.RespostaReativarZona)
}

func (comando *ReativarZona) RespostaReativarZona(super *ComandoCentral, cmd int, payload []byte) {
    if cmd != 0xf0fe {
        fmt.Printf("ReativarZona: resp inesperada %04x\n", cmd)
        super.Bye()
        return
    }

    super.Despedida()
}

func NewReativarZona(zona int) (ComandoCentralSub, string) {
    comando := new(ReativarZona)
    if zona < 1 || zona > 254 {
        return nil, "Zona precisa ser especificada e estar na faixa 1-254"
    }
    comando.zona = byte(zona)
    return comando, ""
}

// Limpar registro de alarme disparado
type LimparDisparo struct {
}

func (comando *LimparDisparo) Autenticado(super *ComandoCentral) {
    pacote := PacoteIsecNet2(0x4013, nil)
    super.EnviarPacote(pacote, comando.RespostaLimparDisparo)
}

func (comando *LimparDisparo) RespostaLimparDisparo(super *ComandoCentral, cmd int, payload []byte) {
    if cmd != 0xf0fe {
        fmt.Printf("LimparDisparo: resp inesperada %04x\n", cmd)
        super.Bye()
        return
    }

    super.Despedida()
}

func NewLimparDisparo(_ int) (ComandoCentralSub, string) {
    comando := new(LimparDisparo)
    return comando, ""
}

// Lista de comandos disponíveis

var Subcomandos map[string]DescComandoSub

func init() {
    Subcomandos = map[string]DescComandoSub{
        "nulo": DescComandoSub{"", false, NewComandoNulo},
        "status": DescComandoSub{"", false, NewSolicitarStatus},
        "ativar": DescComandoSub{"[partição] (se omitida, ativa todas)", true, NewAtivarCentral},
        "desativar": DescComandoSub{"[partição] (se omitida, desativa todas)", true, NewDesativarCentral},
        "desligarsirene": DescComandoSub{"[partição] (se omitida, desliga todas)", true, NewDesligarSirene},
        "limpardisparo": DescComandoSub{"", false, NewLimparDisparo},
        "bypass": DescComandoSub{"<zona> (obrigatório especificar zona)", true, NewBypassZona},
        "cancelbypass": DescComandoSub{"<zona> (obrigatório especificar zona)", true, NewReativarZona},
    }
}
