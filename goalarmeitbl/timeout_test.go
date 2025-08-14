package goalarmeitbl

import (
    "testing"
    "time"
)

func TestTimeout(t *testing.T) {
    cbch := make(chan Event)
    NewTimeout(1 * time.Second, 0, cbch, "foo")
    deadline := time.NewTimer(10 * time.Second)
    for {
        select {
        case evt := <-cbch:
            if evt.Name == "foo" {
                return
            }
            t.Error("Failed")
        case <-deadline.C:
            t.Error("Failed")
        }
    }
}
