package goalarmeitbl

import (
	"fmt"
	"slices"
	"strings"
)

// Construtor de uma implementação/subclasse
type Constructor func(int) (ComandoCentralSub, string)

// Descritor de uma subclasse, para usar num mapa string -> descritor
type DescComandoSub struct {
	ExtraHelp  string
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
	return bits_para_numeros_desde(octetos, inverso, 1)
}

// Como bits_para_numeros, mas o bit 0 do primeiro octeto corresponde ao número "primeiro"
func bits_para_numeros_desde(octetos []byte, inverso bool, primeiro int) string {
	lista := []string{}
	for i, octeto := range octetos {
		for j := range 8 {
			bit := (octeto & (1 << j))
			if (bit != 0 && !inverso) || (bit == 0 && inverso) {
				lista = append(lista, fmt.Sprintf("%d", (primeiro+j+i*8)))
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
		super.Bye(true)
		return
	}

	// Último campo interpretado é payload[63:65], após o octeto prefixado abaixo
	if len(payload) < 64 {
		fmt.Printf("RespostaStatus: resposta curta demais (%d octetos)\n", len(payload))
		super.Bye(true)
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
	fmt.Printf("\t %s\n", armado[int(((payload[21]>>5)&0x03))])
	fmt.Printf("\tZonas em alarme: %s\n", sim_nao(int(payload[21]&0x8)))
	fmt.Printf("\tZonas canceladas: %s\n", sim_nao(int(payload[21]&0x10)))
	fmt.Printf("\tTodas zonas fechadas: %s\n", sim_nao(int(payload[21]&0x4)))
	fmt.Printf("\tSirene: %s\n", sim_nao(int(payload[21]&0x2)))
	fmt.Printf("\tProblemas: %s\n", sim_nao(int(payload[21]&0x1)))
	for particao := range 17 {
		habilitado := payload[22+particao] & 0x80
		if habilitado == 0 {
			continue
		}
		fmt.Printf("Partição %02d:\n", particao)
		fmt.Printf("\tStay: %s\n", sim_nao(int(payload[22+particao]&0x40)))
		fmt.Printf("\tDelay de saída: %s\n", sim_nao(int(payload[22+particao]&0x20)))
		fmt.Printf("\tPronto para armar: %s\n", sim_nao(int(payload[22+particao]&0x10)))
		fmt.Printf("\tAlame ocorreu: %s\n", sim_nao(int(payload[22+particao]&0x08)))
		fmt.Printf("\tEm alarme: %s\n", sim_nao(int(payload[22+particao]&0x04)))
		fmt.Printf("\tArmado modo stay: %s\n", sim_nao(int(payload[22+particao]&0x02)))
		fmt.Printf("\tArmado: %s\n", sim_nao(int(payload[22+particao]&0x01)))
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

// Solicita status das partições (comando 0B20)
type SolicitarParticoes struct {
}

func (comando *SolicitarParticoes) Autenticado(super *ComandoCentral) {
	// 0xff = todas as partições
	pacote := PacoteIsecNet2(0x0b20, []byte{0xff})
	super.EnviarPacote(pacote, comando.RespostaParticoes)
}

func (comando *SolicitarParticoes) RespostaParticoes(super *ComandoCentral, cmd int, payload []byte) {
	if cmd != 0x0b20 {
		fmt.Printf("RespostaParticoes: resp inesperada %04x\n", cmd)
		super.Bye(true)
		return
	}

	// byte 1: 0xff (todas as partições), seguido de um octeto de status por partição, a partir da 1
	if len(payload) < 2 || payload[0] != 0xff {
		fmt.Printf("RespostaParticoes: resposta inválida %s\n", HexPrint(payload))
		super.Bye(true)
		return
	}

	fmt.Println()
	fmt.Println()
	fmt.Println("*******************************************")
	for i, status := range payload[1:] {
		if status&0x80 == 0 {
			// partição desabilitada
			continue
		}
		fmt.Printf("Partição %02d:\n", i+1)
		fmt.Printf("\tArmado: %s\n", sim_nao(int(status&0x01)))
		fmt.Printf("\tArmado modo stay: %s\n", sim_nao(int(status&0x02)))
		fmt.Printf("\tEm alarme: %s\n", sim_nao(int(status&0x04)))
		fmt.Printf("\tAlarme ocorreu: %s\n", sim_nao(int(status&0x08)))
		fmt.Printf("\tPronto para armar: %s\n", sim_nao(int(status&0x10)))
		fmt.Printf("\tDelay de saída: %s\n", sim_nao(int(status&0x20)))
		fmt.Printf("\tDelay de entrada: %s\n", sim_nao(int(status&0x40)))
	}
	fmt.Println("*******************************************")
	fmt.Println()

	super.Despedida()
}

func NewSolicitarParticoes(_ int) (ComandoCentralSub, string) {
	comando := new(SolicitarParticoes)
	return comando, ""
}

// Solicita lista de dispositivos sem fio cadastrados (comando 0B50)
type SolicitarDispositivos struct {
}

func (comando *SolicitarDispositivos) Autenticado(super *ComandoCentral) {
	pacote := PacoteIsecNet2(0x0b50, nil)
	super.EnviarPacote(pacote, comando.RespostaDispositivos)
}

func (comando *SolicitarDispositivos) RespostaDispositivos(super *ComandoCentral, cmd int, payload []byte) {
	if cmd != 0x0b50 {
		fmt.Printf("RespostaDispositivos: resp inesperada %04x\n", cmd)
		super.Bye(true)
		return
	}

	// Último campo interpretado é o octeto 28 (PGMs 13-16)
	if len(payload) < 28 {
		fmt.Printf("RespostaDispositivos: resposta curta demais (%d octetos)\n", len(payload))
		super.Bye(true)
		return
	}

	// Octeto 26: repetidores 1-4 (bits 0-3), PGMs 1-4 (bits 4-7)
	// Octeto 27: PGMs 5-12; octeto 28: PGMs 13-16 (bits 0-3)
	pgms := uint16(payload[25]>>4) | uint16(payload[26])<<4 | uint16(payload[27]&0x0f)<<12

	fmt.Println()
	fmt.Println()
	fmt.Println("*******************************************")
	fmt.Println("Dispositivos sem fio cadastrados:")
	// controles remotos são numerados a partir de 0
	fmt.Printf("\tControles remotos: %s\n", bits_para_numeros_desde(payload[0:13], false, 0))
	fmt.Printf("\tSensores (zonas): %s\n", bits_para_numeros(payload[13:21], false))
	fmt.Printf("\tTeclados: %s\n", bits_para_numeros(payload[21:23], false))
	fmt.Printf("\tSirenes: %s\n", bits_para_numeros(payload[23:25], false))
	fmt.Printf("\tRepetidores: %s\n", bits_para_numeros([]byte{payload[25] & 0x0f}, false))
	fmt.Printf("\tPGMs: %s\n", bits_para_numeros([]byte{byte(pgms), byte(pgms >> 8)}, false))
	fmt.Println("*******************************************")
	fmt.Println()

	super.Despedida()
}

func NewSolicitarDispositivos(_ int) (ComandoCentralSub, string) {
	comando := new(SolicitarDispositivos)
	return comando, ""
}

// Solicita status de problemas da central (comando 0B30)
type SolicitarProblemas struct {
}

func (comando *SolicitarProblemas) Autenticado(super *ComandoCentral) {
	pacote := PacoteIsecNet2(0x0b30, nil)
	super.EnviarPacote(pacote, comando.RespostaProblemas)
}

func (comando *SolicitarProblemas) RespostaProblemas(super *ComandoCentral, cmd int, payload []byte) {
	if cmd != 0x0b30 {
		fmt.Printf("RespostaProblemas: resp inesperada %04x\n", cmd)
		super.Bye(true)
		return
	}

	if len(payload) < 2 {
		fmt.Printf("RespostaProblemas: resposta curta demais (%d octetos)\n", len(payload))
		super.Bye(true)
		return
	}

	fmt.Println()
	fmt.Println()
	fmt.Println("*******************************************")
	fmt.Println("Problemas:")
	fmt.Printf("\tFalta de energia AC: %s\n", sim_nao(int(payload[0]&0x01)))
	fmt.Printf("\tBateria baixa: %s\n", sim_nao(int(payload[0]&0x02)))
	fmt.Printf("\tBateria não conectada: %s\n", sim_nao(int(payload[0]&0x04)))
	fmt.Printf("\tCurto-circuito na bateria: %s\n", sim_nao(int(payload[0]&0x08)))
	fmt.Printf("\tCorte da linha telefônica: %s\n", sim_nao(int(payload[0]&0x10)))
	fmt.Printf("\tFalha de comunicação: %s\n", sim_nao(int(payload[0]&0x20)))
	fmt.Printf("\tFalha de RF: %s\n", sim_nao(int(payload[1]&0x01)))
	// bits 6 e 7 do octeto 1 e 1-7 do octeto 2 não são documentados
	// (o exemplo da planilha sugere que o bit 7 seja falha de conexão GPRS)
	if payload[0]&0xc0 != 0 || payload[1]&0xfe != 0 {
		fmt.Printf("\tOutros bits não documentados: %s\n", HexPrint(payload[0:2]))
	}
	fmt.Println("*******************************************")
	fmt.Println()

	super.Despedida()
}

func NewSolicitarProblemas(_ int) (ComandoCentralSub, string) {
	comando := new(SolicitarProblemas)
	return comando, ""
}

// Lê os eventos mais recentes do buffer de eventos da central
// Primeiro lê o ponteiro de escrita (comando 3003), depois os eventos (comando 3900)
type LerEventos struct {
}

// Buffer circular de 512 eventos; cada leitura traz no máximo 8
const tamanho_buffer_eventos = 512
const eventos_por_leitura = 8

// Índice (2 octetos) + evento (13 octetos)
const tamanho_entrada_evento = 15

func (comando *LerEventos) Autenticado(super *ComandoCentral) {
	pacote := PacoteIsecNet2(0x3003, []byte{0x00})
	super.EnviarPacote(pacote, comando.RespostaPonteiro)
}

func (comando *LerEventos) RespostaPonteiro(super *ComandoCentral, cmd int, payload []byte) {
	if cmd != 0x3003 {
		fmt.Printf("RespostaPonteiro: resp inesperada %04x\n", cmd)
		super.Bye(true)
		return
	}

	// octeto 1: índice (0x00), octetos 2-3: ponteiro de escrita
	if len(payload) < 3 {
		fmt.Printf("RespostaPonteiro: resposta curta demais (%d octetos)\n", len(payload))
		super.Bye(true)
		return
	}

	// O ponteiro aponta para a próxima posição a ser gravada;
	// pede os eventos anteriores, do mais recente para o mais antigo
	ponteiro := int(payload[1])<<8 | int(payload[2])
	indices := []byte{}
	for i := range eventos_por_leitura {
		indice := (ponteiro - 1 - i + tamanho_buffer_eventos) % tamanho_buffer_eventos
		indices = append(indices, byte(indice>>8), byte(indice))
	}

	pacote := PacoteIsecNet2(0x3900, indices)
	super.EnviarPacote(pacote, comando.RespostaEventos)
}

// Interpreta um evento do buffer (13 octetos, sem o índice)
func descrever_evento(e []byte) string {
	data := fmt.Sprintf("20%02x-%02x-%02x %02x:%02x:%02x", e[0], e[1], e[2], e[3], e[4], e[5])
	qualificador := int(e[6] >> 4)
	// dígitos nos nibbles, no formato Contact-ID (0xA = 0)
	codigo, err1 := ContactIDDecode([]byte{e[6] & 0x0f, e[7] >> 4, e[7] & 0x0f})
	zona, err2 := ContactIDDecode([]byte{e[9] & 0x0f, e[10] >> 4, e[10] & 0x0f})
	particao, err3 := ContactIDDecode([]byte{e[11] >> 4, e[11] & 0x0f})
	if err1 != nil || err2 != nil || err3 != nil {
		return fmt.Sprintf("%s evento ilegível %s", data, HexPrint(e))
	}

	detalhes := fmt.Sprintf("evento %03d qualif. %d partição %02d zona/usuário %03d", codigo, qualificador, particao, zona)
	// código programado pelo usuário; 0xFFF = usa o código interno
	if e[8] != 0xff || e[9]>>4 != 0x0f {
		detalhes += fmt.Sprintf(" programado %x%x%x", e[8]>>4, e[8]&0x0f, e[9]>>4)
	}
	if e[12] != 0 {
		detalhes += " com foto"
	}

	descricao, conhecido := DescricaoContactID(codigo, qualificador, particao, zona)
	if !conhecido {
		descricao = "Evento desconhecido"
	}
	return fmt.Sprintf("%s %s (%s)", data, descricao, detalhes)
}

func (comando *LerEventos) RespostaEventos(super *ComandoCentral, cmd int, payload []byte) {
	if cmd != 0x3900 {
		fmt.Printf("RespostaEventos: resp inesperada %04x\n", cmd)
		super.Bye(true)
		return
	}

	if len(payload) == 0 || len(payload)%tamanho_entrada_evento != 0 {
		fmt.Printf("RespostaEventos: tamanho de resposta inesperado (%d octetos)\n", len(payload))
		super.Bye(true)
		return
	}

	fmt.Println()
	fmt.Println()
	fmt.Println("*******************************************")
	fmt.Println("Eventos mais recentes:")
	for i := 0; i < len(payload); i += tamanho_entrada_evento {
		fmt.Printf("\t%s\n", descrever_evento(payload[i+2:i+tamanho_entrada_evento]))
	}
	fmt.Println("*******************************************")
	fmt.Println()

	super.Despedida()
}

func NewLerEventos(_ int) (ComandoCentralSub, string) {
	comando := new(LerEventos)
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
		super.Bye(true)
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
		super.Bye(true)
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
		super.Bye(true)
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
		super.Bye(true)
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
		super.Bye(true)
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
		super.Bye(true)
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
		"nulo":           DescComandoSub{"", false, NewComandoNulo},
		"status":         DescComandoSub{"", false, NewSolicitarStatus},
		"particoes":      DescComandoSub{"", false, NewSolicitarParticoes},
		"dispositivos":   DescComandoSub{"", false, NewSolicitarDispositivos},
		"problemas":      DescComandoSub{"", false, NewSolicitarProblemas},
		"eventos":        DescComandoSub{"", false, NewLerEventos},
		"ativar":         DescComandoSub{"[partição] (se omitida, ativa todas)", true, NewAtivarCentral},
		"desativar":      DescComandoSub{"[partição] (se omitida, desativa todas)", true, NewDesativarCentral},
		"desligarsirene": DescComandoSub{"[partição] (se omitida, desliga todas)", true, NewDesligarSirene},
		"limpardisparo":  DescComandoSub{"", false, NewLimparDisparo},
		"bypass":         DescComandoSub{"<zona> (obrigatório especificar zona)", true, NewBypassZona},
		"cancelbypass":   DescComandoSub{"<zona> (obrigatório especificar zona)", true, NewReativarZona},
	}
}
