package goalarmeitbl

import (
    "fmt"
    "time"
    "os/exec"
    "sync"
)

type ReceptorIP struct {
    tcp *TCPServer
    cfg ReceptorIPConfig
    wg sync.WaitGroup
    centrais_conectadas int
    cnc_alarme bool
}

func NewReceptorIP(cfg ReceptorIPConfig) (*ReceptorIP, error) {
    r := new(ReceptorIP)
    r.cfg = cfg
    var err error
    r.tcp, err = NewTCPServer(fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port))
    if err != nil {
        return r, err
    }
    fmt.Println("ReceptorIP: inicio")

    r.wg.Go(func() {
        r.tcp.Timeout(15 * time.Second, 0, "Watchdog")
        r.tcp.Timeout(3600 * time.Second, 0, "Central_nc")

        for evt := range r.tcp.Events {
            switch evt.Name {
            case "new":
                r.centrais_conectadas += 1
                NewTratadorReceptorIP(r, evt.Cargo.(*TCPSession))
                fmt.Printf("ReceptorIP: %d centrais conectadas\n", r.centrais_conectadas)
            case "closed":
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

func (r *ReceptorIP) InvocaGancho(tipo string, msg string) {
    script := r.cfg.Ganchos["gancho_" + tipo]
    cmd := exec.Command(script, msg)
    if err := cmd.Run(); err != nil {
        fmt.Printf("ReceptorIP: script %s %s falhou com erro %v\n", tipo, script, err) 
    } else {
        fmt.Printf("ReceptorIP: script %s %s executado com sucesso\n", tipo, script)
    }
}

func (r *ReceptorIP) Watchdog(to *Timeout) {
    fmt.Println("receptor em funcionamento")
    r.InvocaGancho("watchdog", "")
    to.Reset(3600 * time.Second, 0)
}

func (r *ReceptorIP) CentralNaoConectada(to *Timeout) {
    if r.centrais_conectadas <= 0 {
        if !r.cnc_alarme {
            r.cnc_alarme = true
            fmt.Println("nenhuma central conectada")
            r.InvocaGancho("central", ", 1")
        }
    } else {
        if r.cnc_alarme {
            r.cnc_alarme = false
            r.InvocaGancho("central", ", 0")
        }
    }
    to.Restart()
}
