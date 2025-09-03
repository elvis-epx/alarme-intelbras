package goalarmeitbl

import (
    "time"
    "math/rand/v2"
    "sync"
    "fmt"
)

// Base type of user-facing event loops
type Event struct {
    Name string
    Cargo any
}

type Timeout struct {
    mutex sync.Mutex
    parent *Parent

    avgto time.Duration
    fudge time.Duration
    impl *time.Timer
    alive bool
    eta time.Time

    cbch chan Event
    cbchmsg string
}

func NewTimeout(avgto time.Duration, fudge time.Duration, cbch chan Event, cbchmsg string, parent *Parent) (*Timeout) {
    timeout := new(Timeout)
    timeout.parent = parent
    timeout.avgto = avgto
    timeout.fudge = fudge
    timeout.eta = time.Now()
    timeout.cbch = cbch
    timeout.cbchmsg = cbchmsg

    // add to parent here so we are sure this happens before timeout is started and potentially emits any events
    if parent != nil {
        parent.Adopt(timeout)
    }

    timeout._restart()

    return timeout
}

func (timeout *Timeout) _restart() {
    if timeout.impl != nil {
        timeout.impl.Stop()
    }

    relative_eta := timeout.avgto + 2 * timeout.fudge * time.Duration(rand.Float32() - 0.5)
    timeout.eta = time.Now().Add(relative_eta)
    timeout.alive = true

    timeout.impl = time.AfterFunc(relative_eta, func() {
        timeout.mutex.Lock()
        defer timeout.mutex.Unlock()
        timeout.alive = false
        timeout.cbch <- Event{timeout.cbchmsg, timeout}
    })
}

func (timeout *Timeout) _stop() {
    timeout.impl.Stop()
    timeout.alive = false
}

// public methods for Timeout

// Stop timeout but allow for Restart later
func (timeout *Timeout) Stop() {
    timeout.mutex.Lock()
    defer timeout.mutex.Unlock()

    timeout._stop()
}

// Restart timeout with the same parameters
func (timeout *Timeout) Restart() {
    timeout.mutex.Lock()
    defer timeout.mutex.Unlock()

    timeout._restart()
}

// Restart timemout with new parameters
func (timeout *Timeout) Reset(avgto time.Duration, fudge time.Duration) {
    timeout.mutex.Lock()
    defer timeout.mutex.Unlock()

    timeout.avgto = avgto
    timeout.fudge = fudge
    timeout._restart()
}

// Stop and free timeout. This timeout won't post events after the call returns.
func (timeout *Timeout) Free() {
    timeout.mutex.Lock()
    defer timeout.mutex.Unlock()

    if timeout.parent != nil {
        timeout.parent.Disown(timeout)
        timeout.parent = nil
    }
    timeout._stop()
}

// Called by Parent.DisownAll() - involuntary mass disown of all children
func (timeout *Timeout) Disowned() {
    timeout.mutex.Lock()
    defer timeout.mutex.Unlock()

    timeout.parent = nil
    timeout._stop()
}

// Returns a unique ChildId
func (timeout *Timeout) GetChildId() ChildId {
    return ChildId(fmt.Sprintf("%p", timeout))
}

func (timeout *Timeout) Alive() (bool) {
    timeout.mutex.Lock()
    defer timeout.mutex.Unlock()

    return timeout.alive
}

func (timeout *Timeout) Remaining() (time.Duration) {
    timeout.mutex.Lock()
    defer timeout.mutex.Unlock()

    if !timeout.alive {
        return 0
    }
    return timeout.eta.Sub(time.Now()) 
}
