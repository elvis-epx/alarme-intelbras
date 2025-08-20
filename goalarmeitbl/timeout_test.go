package goalarmeitbl

import (
    "testing"
    "time"
)

func TestTimeout(t *testing.T) {
    cbch := make(chan Event)
    NewTimeout(1 * time.Second, 0, cbch, "foo")
    lower_deadline := time.Now().Add(1 * time.Second)
    upper_deadline := time.NewTimer(2 * time.Second)
loop:
    for {
        select {
        case evt := <-cbch:
            now := time.Now()
            if now.After(lower_deadline) && evt.Name == "foo" {
                break loop
            }
            t.Error("Failed", now, lower_deadline)
            return
        case <-upper_deadline.C:
            t.Error("Failed")
            return
        }
    }
}

func TestTimeout2(t *testing.T) {
    cbch := make(chan Event)
    to := NewTimeout(1 * time.Second, 0, cbch, "foo")
    lower_deadline := time.Now().Add(3 * time.Second)
    upper_deadline := time.NewTimer(5 * time.Second)
    to.Reset(3 * time.Second, 0)
loop:
    for {
        select {
        case evt := <-cbch:
            if time.Now().After(lower_deadline) && evt.Name == "foo" {
                break loop
            }
            t.Error("Failed")
            return
        case <-upper_deadline.C:
            t.Error("Failed")
            return
        }
    }
    to.Stop()
    to.Free()
}
