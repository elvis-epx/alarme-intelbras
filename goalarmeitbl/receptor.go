package goalarmeitbl

import (
    "context"
    "fmt"
    "time"
    "os/exec"
    "sync"
)

// Tempo máximo de execução de um script de gancho
const timeoutGancho = 120 * time.Second

// Profundidade da fila de ganchos pendentes
const filaGanchos = 256

type invocacaoGancho struct {
    tipo string
    args []string
}

type ReceptorIP struct {
    tcp *TCPServer
    cfg ReceptorIPConfig
    wg sync.WaitGroup
    centrais_conectadas int
    cnc_alarme bool
    ganchos chan invocacaoGancho
}

func NewReceptorIP(cfg ReceptorIPConfig) (*ReceptorIP, error) {
    r := new(ReceptorIP)
    r.cfg = cfg
    r.ganchos = make(chan invocacaoGancho, filaGanchos)
    var err error
    r.tcp, err = NewTCPServer(fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port))
    if err != nil {
        return r, err
    }
    fmt.Println("ReceptorIP: inicio")

    // Ganchos são executados em série, numa goroutine dedicada, para que
    // a) scripts lentos não bloqueiem a comunicação com as centrais, e
    // b) a ordem dos eventos seja preservada
    go func() {
        for g := range r.ganchos {
            r.executaGancho(g.tipo, g.args)
        }
    }()

    r.wg.Go(func() {
        r.tcp.Timeout(15 * time.Second, 0, "Watchdog")
        r.tcp.Timeout(3600 * time.Second, 0, "Central_nc")

        for evt := range r.tcp.Events {
            switch evt.Name {
            case "New":
                r.centrais_conectadas += 1
                NewTratadorReceptorIP(r, evt.Cargo.(*TCPSession))
                fmt.Printf("ReceptorIP: %d centrais conectadas\n", r.centrais_conectadas)
            case "Closed":
                r.centrais_conectadas -= 1
                fmt.Printf("ReceptorIP: %d centrais conectadas\n", r.centrais_conectadas)
            case "Watchdog":
                r.Watchdog(evt.Cargo.(*Timeout))
            case "Central_nc":
                r.CentralNaoConectada(evt.Cargo.(*Timeout))
            }
        }

        fmt.Println("ReceptorIP: fim ----")
    })

    return r, nil
}

func (r *ReceptorIP) Wait() {
    r.wg.Wait()
}

// Enfileira a invocação de um script de gancho. Não espera a execução.
func (r *ReceptorIP) InvocaGancho(tipo string, args ...string) {
    r.ganchos <- invocacaoGancho{tipo, args}
}

func (r *ReceptorIP) executaGancho(tipo string, args []string) {
    script := r.cfg.Ganchos["gancho_" + tipo]
    ctx, cancel := context.WithTimeout(context.Background(), timeoutGancho)
    defer cancel()
    cmd := exec.CommandContext(ctx, script, args...)
    if err := cmd.Run(); err != nil {
        if ctx.Err() != nil {
            fmt.Printf("ReceptorIP: script %s %s abortado por timeout\n", tipo, script)
        } else {
            fmt.Printf("ReceptorIP: script %s %s falhou com erro %v\n", tipo, script, err)
        }
    }
}

func (r *ReceptorIP) Watchdog(to *Timeout) {
    fmt.Println("receptor em funcionamento")
    r.InvocaGancho("watchdog")
    to.Reset(3600 * time.Second, 0)
}

func (r *ReceptorIP) CentralNaoConectada(to *Timeout) {
    if r.centrais_conectadas <= 0 {
        if !r.cnc_alarme {
            r.cnc_alarme = true
            fmt.Println("nenhuma central conectada")
            r.InvocaGancho("central", "1")
        }
    } else {
        if r.cnc_alarme {
            r.cnc_alarme = false
            r.InvocaGancho("central", "0")
        }
    }
    to.Restart()
}
