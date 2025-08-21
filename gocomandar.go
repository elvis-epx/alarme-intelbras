package main

import (
    "os"
    "log"
    "strconv"
    "github.com/elvis-epx/alarme-intelbras/goalarmeitbl"
)

type Observador struct {
}

func (o *Observador) Resultado(res int) {
    if (res == 0) {
        log.Print("Sucesso")
    } else {
        log.Print("Fracasso")
    }
}

func main() {
    comando := os.Args[1]
    serveraddr := os.Args[2]

    senha, err := strconv.Atoi(os.Args[3])
    if err != nil {
        log.Fatal("Senha inválida")
    }

    tam_senha, err2 := strconv.Atoi(os.Args[4])
    if err2 != nil || (tam_senha != 4 && tam_senha != 6) {
        log.Fatal("Tamanho senha inválida")
    }

    descritor, ok := goalarmeitbl.Subcomandos[comando]
    if !ok {
        log.Fatal("Comando não reconhecido")
    }

    extra := 0
    if descritor.ExtraParam {
        if len(os.Args) > 5 {
            extra, err = strconv.Atoi(os.Args[5])
            if err != nil || extra > 255 || extra < 0 {
                log.Fatal("Valor extra inválido")
            }
        }
    }

    sub := descritor.Construtor(extra)
    c := goalarmeitbl.NewComandoCentral(sub, new(Observador), serveraddr, senha, tam_senha)
    c.Wait()
}

