package goalarmeitbl

import (
    "fmt"
    "errors"
    "io"
    "github.com/bigkevmcd/go-configparser"
)

type ReceptorIPConfig struct {
    Ganchos map[string]string
    Addr string
    Port int
}

func NewReceptorIPConfig(in io.Reader) (ReceptorIPConfig, error) {
    sec := "receptorip"
    ganchos := []string{"gancho_arquivo", "gancho_central", "gancho_ev", "gancho_msg", "gancho_watchdog"}
    c := ReceptorIPConfig{make(map[string]string), "", 9010}

    p, err := configparser.ParseReaderWithOptions(in)
    if err != nil {
        return c, err
    }

    if !p.HasSection(sec) {
        return c, errors.New(fmt.Sprintf("Seção [%s] não encontrada na config", sec))
    }

    addr, err := p.Get(sec, "addr")
    if err == nil {
        if addr == "0.0.0.0" || addr == "::" {
            fmt.Println("Aviso: addr coringa, ouvindo em todas as interfaces")
            addr = ""
        }
        c.Addr = addr
    } else {
        fmt.Println("Aviso: addr não especificado, ouvindo em todas as interfaces")
    }

    port, err := p.GetInt64(sec, "port")
    if err == nil {
        if port <= 0 || port >= 65536 {
            fmt.Printf("Aviso: port com valor inválido, ouvindo na porta default %d\n", c.Port)
        } else {
            c.Port = int(port)
        }
    } else {
        fmt.Printf("Aviso: port não especificado, ouvindo na porta default %d\n", c.Port)
    }

    for _, gancho := range ganchos {
        script, err := p.Get(sec, gancho)
        if err != nil {
            return c, errors.New(fmt.Sprintf("%s não encontrado na config", gancho))
        }
        c.Ganchos[gancho] = script
    }

    return c, nil
}
