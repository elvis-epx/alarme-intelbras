package goalarmeitbl

import (
	"encoding/hex"
)

// Calcula checksum de frame longo
// Presume que "dados" contém o byte de comprimento mas não contém o byte de checksum
func Checksum(dados []byte) byte {
    chksum := uint8(0)
    for _, n := range dados {
        chksum = chksum ^ n
    }
    chksum = chksum ^ uint8(0xff)
    return chksum
}

func HexPrint(buf []byte) string {
    s := hex.EncodeToString(buf)
    ss := ""
    for i := 0; i < len(s) - 2; i += 2 {
        ss += s[i:i+2]
        ss += " "
    }
    if len(s) > 0 {
        ss += s[len(s)-2:]
    }
    return ss
}
