package goalarmeitbl

import (
    "testing"
    "time"
)

func TestTimeout(t *testing.T) {
    cbch := make(chan Event)
    lower_deadline := time.Now().Add(1 * time.Second)
    lower_deadline2 := time.NewTimer(500 * time.Millisecond)
    to := NewTimeout(1 * time.Second, 0, cbch, "foo", nil)
    upper_deadline := time.NewTimer(2 * time.Second)
loop:
    for {
        select {
        case evt := <-cbch:
            now := time.Now()
            if now.After(lower_deadline) && evt.Name == "foo" {
                if to.Alive() {
                    t.Error("alive false Failed")
                }
                if to.Remaining() != 0 {
                    t.Error("remaining 0 Failed")
                }
                break loop
            }
            t.Error("Failed", now, lower_deadline, evt.Name)
            return
        case <-upper_deadline.C:
            t.Error("Failed")
            return
        case <-lower_deadline2.C:
            if to.Remaining() <= 0 {
                t.Error("remaining >0 Failed")
                return
            }
            if !to.Alive() {
                t.Error("alive true Failed")
            }
        }
    }
}

func TestTimeout2(t *testing.T) {
    cbch := make(chan Event)
    lower_deadline := time.Now().Add(3 * time.Second)
    to := NewTimeout(1 * time.Second, 0, cbch, "foo", nil)
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
