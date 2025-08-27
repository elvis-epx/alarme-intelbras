package main

import (
    "log"
    // "io"
    "os"
    "fmt"
    "github.com/elvis-epx/alarme-intelbras/goalarmeitbl"
)

func usage(err string) {
    fmt.Printf("Erro: %s\n", err)
    fmt.Printf("Uso: %s <arquivo de configuração>\n", os.Args[0])
    os.Exit(3)
}

func main() {
    // log.SetOutput(io.Discard)
    if len(os.Args) < 2 {
        usage("arquivo de configuração não especificado")
    }
    f, err := os.Open(os.Args[1])
    if err != nil {
        log.Print(err)
        usage("Arquivo de configuração não pôde ser aberto")
    }
    cfg, err := goalarmeitbl.NewReceptorIPConfig(f)
    if err != nil {
        usage(fmt.Sprintf("Arquivo de configuração inválido: %v", err))
    }
    f.Close()
    srv, err := goalarmeitbl.NewReceptorIP(cfg)
    if err != nil {
        usage(fmt.Sprintf("Falha ao iniciar receptor IP: %v", err))
    }
    srv.Wait()
}
