package goalarmeitbl

import (
    "log"
    "slices"
    "strings"
    "fmt"
)

type ComandoNulo struct {
    Super *ComandoCentral
}

func (comando *ComandoNulo) Autenticado() {
    comando.Super.Despedida()
}

func (comando *ComandoNulo) Wait() {
    comando.Super.Wait()
}

func NewComandoNulo(observer ObserverComando, serveraddr string, senha int, tam_senha int, extra int) *ComandoNulo {
    comando := new(ComandoNulo)
    comando.Super = NewComandoCentral(comando, observer, serveraddr, senha, tam_senha, extra)
    log.Print("ComandoNulo: inicio")
    return comando
}


type SolicitarStatus struct {
    Super *ComandoCentral
}

func (comando *SolicitarStatus) Autenticado() {
    pacote := PacoteIsecNet2(0x0b4a, nil)
    comando.Super.EnviarPacote(pacote, comando.RespostaStatus)
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

func (comando *SolicitarStatus) RespostaStatus(cmd int, payload []byte) {
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
    log.Printf("\t" + armado[int(((payload[21] >> 5) & 0x03))])
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
    log.Printf("Zonas abertas:" + bits_para_numeros(payload[39:47], false))
    log.Printf("Zonas em alarme:" + bits_para_numeros(payload[47:55], false))
    // log.Printf("Zonas ativas:" + bits_para_numeros(payload[55:63], true))
    log.Printf("Zonas em bypass:" + bits_para_numeros(payload[55:63], false))
    log.Printf("Sirenes ligadas:" + bits_para_numeros(payload[63:65], false))

    // TODO interpretar mais campos
    log.Print("*******************************************")
    log.Print()

    comando.Super.Despedida()
}

func (comando *SolicitarStatus) Wait() {
    comando.Super.Wait()
}

func NewSolicitarStatus(observer ObserverComando, serveraddr string, senha int, tam_senha int, extra int) *SolicitarStatus {
    comando := new(SolicitarStatus)
    comando.Super = NewComandoCentral(comando, observer, serveraddr, senha, tam_senha, extra)
    log.Print("SolicitarStatus: inicio")
    return comando
}
