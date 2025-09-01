package goalarmeitbl

import (
    "time"
    "math/rand/v2"
    "log"
    "sync"
)

// Base type of user-facing event loops
type Event struct {
    Name string
    Cargo any
}

type Timeout struct {
    mutex sync.Mutex
    owner *TimeoutOwner

    avgto time.Duration
    fudge time.Duration
    impl *time.Timer
    alive bool
    eta time.Time

    cbch chan Event
    cbchmsg string
}

func NewTimeout(avgto time.Duration, fudge time.Duration, cbch chan Event, cbchmsg string, owner *TimeoutOwner) (*Timeout) {
    timeout := new(Timeout)
    timeout.owner = owner
    timeout.avgto = avgto
    timeout.fudge = fudge
    timeout.eta = time.Now()
    timeout.cbch = cbch
    timeout.cbchmsg = cbchmsg

    // add to owner here so we are sure this happens before timeout is started and potentially emits any events
    if owner != nil {
        owner.own(timeout)
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

    if timeout.owner != nil {
        timeout.owner.disown(timeout)
        timeout.owner = nil
    }
    timeout._stop()
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

// Timeout owner that is part of a TCPSession or a TCPServer

type TimeoutOwner struct {
    timeouts map[*Timeout]bool
    mutex sync.Mutex
}

func NewTimeoutOwner(cbch chan Event) *TimeoutOwner {
    t := new(TimeoutOwner)
    t.timeouts = make(map[*Timeout]bool)
    return t
}

// Own a timeout. Called by NewTimeout()
func (t *TimeoutOwner) own(to *Timeout) {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    t.timeouts[to] = true
}

// Disown a timeout. Called by Timeout.Free()
func (t *TimeoutOwner) disown(to *Timeout) {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    delete(t.timeouts, to)
    log.Printf("Disowned timeout %p", to)
}

// Synchronously stop and release all owned Timeouts
func (t *TimeoutOwner) Release() {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    for to := range t.timeouts {
        to.Stop() // Timeout won't call disown(), no need to
        log.Printf("Released timeout %p", to)
    }
    t.timeouts = make(map[*Timeout]bool)
}

// Create new owned Timeout
func (t *TimeoutOwner) Timeout(avgto time.Duration, fudge time.Duration, cbch chan Event, cbchmsg string) (*Timeout) {
    return NewTimeout(avgto, fudge, cbch, cbchmsg, t)
}
