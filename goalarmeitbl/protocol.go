package goalarmeitbl

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
