package main

import (
    "fmt"
    "github.com/elvis-epx/alarme-intelbras/goalarmeitbl"
    "log"
    "os"
    "io"
)

func usage(err string) {
    fmt.Printf("Erro: %s\n", err)
    fmt.Printf("Uso: %s <arquivo de configuração>\n", os.Args[0])
    os.Exit(3)
}

func main() {
    if os.Getenv("LOGITBL") != "" {
        log.SetOutput(os.Stderr)
    } else {
        log.SetOutput(io.Discard)
    }

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

    if cfg.LogLevel != "" {
        log.SetOutput(os.Stderr)
    }

    srv, err := goalarmeitbl.NewReceptorIP(cfg)
    if err != nil {
        usage(fmt.Sprintf("Falha ao iniciar receptor IP: %v", err))
    }
    srv.Wait()
}
