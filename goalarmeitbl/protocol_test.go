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
