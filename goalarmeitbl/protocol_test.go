package goalarmeitbl

import (
    "testing"
)

func TestChecksum(t *testing.T) {
    pkt := []byte{0x74, 0x01, 0x35, 0x50, 0x26}
    if Checksum(pkt) != 201 {
        t.Error("Checksum failed")
    }
}

func TestHexPrint(t *testing.T) {
    pkt := []byte{0x74, 0x01, 0x35, 0x50, 0x26}
    res := HexPrint(pkt)
    if res != "74 01 35 50 26" {
        t.Error("TextHexPrint failed I '" + res + "'")
    }

    pkt2 := []byte{0x74}
    res2 := HexPrint(pkt2)
    if res2 != "74" {
        t.Error("TextHexPrint failed II '" + res2 + "'")
    }

    pkt3 := []byte(nil)
    res3 := HexPrint(pkt3)
    if res3 != "" {
        t.Error("TextHexPrint failed III '" + res3 + "'")
    }
}
