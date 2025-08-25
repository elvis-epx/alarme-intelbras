package goalarmeitbl

import (
    "fmt"
    "sync"
    "os/exec"
)

type ReceptorIP struct {
    tcp *TCPServer
    cfg ReceptorIPConfig
    wg sync.WaitGroup
}

func NewReceptorIP(cfg ReceptorIPConfig) (*ReceptorIP, error) {
    r := new(ReceptorIP)
    r.cfg = cfg
    var err error
    r.tcp, err = NewTCPServer(fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port))
    if err != nil {
        return r, err
    }
    r.wg = sync.WaitGroup{}
    r.wg.Add(1)
    fmt.Println("ReceptorIP: inicio")

    go func() {
        for evt := range r.tcp.Events {
            if evt.Name == "new" {
                NewTratadorReceptorIP(r, evt.Cargo.(*TCPSession))
            }
        }
        r.wg.Done()
        fmt.Println("ReceptorIP: fim ----")
    }()

    return r, nil
}

func (r *ReceptorIP) Wait() {
    r.wg.Wait()
}

func (r *ReceptorIP) InvocaGancho(tipo string, msg string) {
    script := r.cfg.Ganchos[tipo]
    cmd := exec.Command(script, msg)
    if err := cmd.Run(); err != nil {
        fmt.Printf("ReceptorIP: script %s %s falhou com erro %v\n", tipo, script, err) 
    } else {
        fmt.Printf("ReceptorIP: script %s %s executado com sucesso\n", tipo, script)
    }
}
