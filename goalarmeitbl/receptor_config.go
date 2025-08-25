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
    c := ReceptorIPConfig{}

    p, err := configparser.ParseReaderWithOptions(in)
    if err != nil {
        return c, err
    }

    if !p.HasSection(sec) {
        return c, errors.New(fmt.Sprintf("Seção [%s] não encontrada na config"))
    }

    addr, err := p.Get(sec, "addr")
    if err != nil {
        return c, errors.New("addr não encontrado na config")
    }
    if addr == "0.0.0.0" || addr == "::" {
        addr = ""
    }
    c.Addr = addr

    port, err := p.GetInt64(sec, "port")
    if err != nil {
        return c, errors.New("port não encontrado na config")
    }
    if port <= 0 {
        port = 9009
    }
    c.Port = int(port)

    for _, gancho := range ganchos {
        script, err := p.Get(sec, gancho)
        if err != nil {
            return c, errors.New(fmt.Sprintf("%s não encontrado na config", gancho))
        }
        c.Ganchos[gancho] = script
    }

    return c, nil
}
