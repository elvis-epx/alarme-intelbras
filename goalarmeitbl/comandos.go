package goalarmeitbl

import (
    "log"
)

type ComandoNulo struct {
    Super *ComandoCentral
}

func (comando *ComandoNulo) Autenticado() {
    comando.Super.Despedida()
}

func NewComandoNulo(observer ObserverComando, serveraddr string, senha int, tam_senha int, extra int) *ComandoNulo {
    comando := new(ComandoNulo)
    comando.Super = NewComandoCentral(comando, observer, serveraddr, senha, tam_senha, extra)
    log.Print("ComandoNulo: inicio")
    return comando
}
