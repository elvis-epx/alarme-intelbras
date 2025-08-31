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

// internal structure to control Timeout safely
type TimeoutControl struct {
    name string
    avgto time.Duration
    fudge time.Duration
}

// internal structure to get Timeout information safely
type TimeoutInfo struct {
    eta time.Time
    alive bool
}

// Timeout struct
type Timeout struct {
    owner *TimeoutOwner
    control chan TimeoutControl // must be bufferless, see Free() and "free" event
    info chan TimeoutInfo
}

// Private parts of Timeout that should not be touched outside the goroutine
type TimeoutPriv struct {
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
    timeout.control = make(chan TimeoutControl)
    timeout.info = make(chan TimeoutInfo)

    priv := new(TimeoutPriv)
    priv.avgto = avgto
    priv.fudge = fudge
    priv.eta = time.Now()
    priv.cbch = cbch
    priv.cbchmsg = cbchmsg

    // add to owner here so we are sure this happens before timeout is started and potentially emits any events
    if owner != nil {
        owner.own(timeout)
    }

    go timeout.handler(priv)
    // makes sure "restart" is the first command the goroutine receives,
    // and any command sent right after the return will have to wait
    defer timeout.Restart()
    return timeout
}

// Only this goroutine (and upstream restart()) can touch private data
func (timeout *Timeout) handler(priv *TimeoutPriv) {
loop:
    for {
        select {
        case cmd := <- timeout.control:
            switch cmd.name {
            case "reset":
                priv.avgto = cmd.avgto
                priv.fudge = cmd.fudge
                timeout.restart(priv)
            case "restart":
                timeout.restart(priv)
            case "trigger":
                priv.alive = false
                priv.cbch <- Event{priv.cbchmsg, timeout}
            case "stop":
                priv.impl.Stop()
                priv.alive = false
            case "free":
                priv.impl.Stop()
                priv.alive = false
                // make sure program will panic if anybody tries to use this afterwards
                close(timeout.control)
                close(timeout.info)
                break loop
            }
        case timeout.info <- TimeoutInfo{priv.eta, priv.alive}:
            continue
        }
    }
}

// called only by goroutine
func (timeout *Timeout) restart(priv *TimeoutPriv) {
    if priv.impl != nil {
        priv.impl.Stop()
    }

    relative_eta := priv.avgto + 2 * priv.fudge * time.Duration(rand.Float32() - 0.5)
    priv.eta = time.Now().Add(relative_eta)
    priv.alive = true

    priv.impl = time.AfterFunc(relative_eta, func() {
        // goroutine context; make sure it goes through the control channel
        timeout.control <- TimeoutControl{"trigger", 0, 0}
    })
}

// public methods for Timeout

// Stop timeout but allow for Restart later
func (timeout *Timeout) Stop() {
    timeout.control <- TimeoutControl{"stop", 0, 0}
}

// Restart timeout with the same parameters
func (timeout *Timeout) Restart() {
    timeout.control <- TimeoutControl{"restart", 0, 0}
}

// Restart timemout with new parameters
func (timeout *Timeout) Reset(avgto time.Duration, fudge time.Duration) {
    timeout.control <- TimeoutControl{"reset", avgto, fudge}
}

// Stop and free timeout. This timeout won't post events after the call returns.
// Non-reentrant, non-idempotent!
// Caller must guarantee the timeout isn't and won't be Free()d by another goroutine
func (timeout *Timeout) Free() {
    if timeout.owner != nil {
        timeout.owner.disown(timeout)
    }
    timeout.free_in()
}

// Called directly by TimeoutOwner upon mass release
func (timeout *Timeout) free_in() {
    // Guarantees that this Timeout won't post events after return
    // because timeout.control is bufferless and "free" will the last command to be processed
    timeout.control <- TimeoutControl{"free", 0, 0}
}

// Returns timeout state
func (timeout *Timeout) Alive() (bool) {
    info := <- timeout.info
    return info.alive
}

// Returns timeout state
func (timeout *Timeout) Remaining() (time.Duration) {
    info := <- timeout.info
    if !info.alive {
        return 0
    }
    return info.eta.Sub(time.Now()) 
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
        to.free_in() // Freeing timeout this way won't call disown() back (would be a deadlock)
        log.Printf("Released timeout %p", to)
    }
    t.timeouts = make(map[*Timeout]bool)
}

// Create new owned Timeout
func (t *TimeoutOwner) Timeout(avgto time.Duration, fudge time.Duration, cbch chan Event, cbchmsg string) (*Timeout) {
    return NewTimeout(avgto, fudge, cbch, cbchmsg, t)
}
