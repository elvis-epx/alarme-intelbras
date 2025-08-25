package main

import (
    "os"
    "fmt"
    "strconv"
    "github.com/elvis-epx/alarme-intelbras/goalarmeitbl"
    // "log"
    // "io"
)

type Observador struct {
}

func (o *Observador) Resultado(res int) {
    if (res == 0) {
        fmt.Println("Sucesso")
    } else {
        fmt.Println("Fracasso")
        os.Exit(2)
    }
}

func usage(err string) {
    fmt.Printf("Uso: %s <endereço:porta> <senha> <tamanho senha> <comando> [partição ou zona]\n", os.Args[0])
    fmt.Println()
    fmt.Println("O parâmetro Partição/Zona pode ou não ser requerido, a depender do comando")
    fmt.Println()
    fmt.Println("Comandos disponíveis")
    fmt.Println("--------------------")
    for comando, descritor := range goalarmeitbl.Subcomandos {
        fmt.Printf("%s %s\n", comando, descritor.ExtraHelp)
    }
    fmt.Println()
    fmt.Printf("Erro: %s\n", err)
    os.Exit(3)
}

func main() {
    // log.SetOutput(io.Discard)
    if len(os.Args) < 5 {
        usage("Forneça os parâmetros necessários")
    }

    serveraddr := os.Args[1]

    senha, err := strconv.Atoi(os.Args[2])
    if err != nil {
        usage("Senha inválida")
    }

    tam_senha, err2 := strconv.Atoi(os.Args[3])
    if err2 != nil || (tam_senha != 4 && tam_senha != 6) {
        usage("Tamanho senha inválida")
    }

    comando := os.Args[4]

    descritor, ok := goalarmeitbl.Subcomandos[comando]
    if !ok {
        usage("Comando não reconhecido")
    }

    extra := 0
    if descritor.ExtraParam {
        if len(os.Args) > 5 {
            extra, err = strconv.Atoi(os.Args[5])
            if err != nil || extra > 255 || extra < 0 {
                usage("Parâmetro extra inválido")
            }
        }
    } else {
        if len(os.Args) > 5 {
            usage("Parâmetro extra desnecessário")
        }
    }

    sub, errstring := descritor.Construtor(extra)
    if errstring != "" {
        usage(errstring)
    }
    
    c := goalarmeitbl.NewComandoCentral(sub, new(Observador), serveraddr, senha, tam_senha)
    c.Wait()
}
