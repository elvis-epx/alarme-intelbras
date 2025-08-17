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

    var c goalarmeitbl.ComandoCentralSub

    extra := 0
    if comando == "ativar" || comando == "desativar" {
        if len(os.Args) > 5 {
            extra, err = strconv.Atoi(os.Args[5])
            if err != nil || extra > 255 || extra < 0 {
                log.Fatal("Valor extra inválido")
            }
        }
    }

    // TODO use something more clever like reflection?
    if comando == "nulo" {
        c = goalarmeitbl.NewComandoNulo(new(Observador), serveraddr, senha, tam_senha)
    } else if comando == "status" {
        c = goalarmeitbl.NewSolicitarStatus(new(Observador), serveraddr, senha, tam_senha)
    } else if comando == "ativar" {
        c = goalarmeitbl.NewAtivarCentral(new(Observador), serveraddr, senha, tam_senha, extra)
    } else if comando == "desativar" {
        c = goalarmeitbl.NewDesativarCentral(new(Observador), serveraddr, senha, tam_senha, extra)
    }
    c.Wait()
}
