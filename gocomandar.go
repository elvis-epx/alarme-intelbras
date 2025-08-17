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
    serveraddr := os.Args[1]

    senha, err := strconv.Atoi(os.Args[2])
    if err != nil {
        log.Fatal("Senha inválida")
    }

    tam_senha, err2 := strconv.Atoi(os.Args[3])
    if err2 != nil || (tam_senha != 4 && tam_senha != 6) {
        log.Fatal("Tamanho senha inválida")
    }

    c := goalarmeitbl.NewComandoNulo(new(Observador), serveraddr, senha, tam_senha, 0)
    c.Super.Wait()
}
