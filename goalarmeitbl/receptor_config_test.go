package goalarmeitbl

import (
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	f := strings.NewReader("moo")
	_, err := NewReceptorIPConfig(f)
	if err == nil {
		t.Error("Should have failed")
	}
}
