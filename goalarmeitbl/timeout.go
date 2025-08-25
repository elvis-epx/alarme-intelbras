package goalarmeitbl

import (
    "time"
    "math/rand/v2"
)

// Base type of user-facing event loops
type Event struct {
    Name string
    Cargo any
}

// Owner of Events channel, if any
type EventsOwner interface {
    ReleaseTimeout(*Timeout)
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
    avgto time.Duration
    fudge time.Duration
    impl *time.Timer
    alive bool
    eta time.Time

    cbch chan Event
    cbchmsg string
    cbchowner EventsOwner

    control chan TimeoutControl // must be bufferless, see Free() and "free" event
    info chan TimeoutInfo
}

type TimeoutCallback func (*Timeout)

func NewTimeout(avgto time.Duration, fudge time.Duration, cbch chan Event, cbchmsg string, cbchowner EventsOwner) (*Timeout) {
    timeout := Timeout{avgto, fudge, nil, false, time.Now(),
                cbch, cbchmsg, cbchowner,
                make(chan TimeoutControl), make(chan TimeoutInfo)}
    go timeout.handler()
    defer timeout.Restart()
    return &timeout
}

func (timeout *Timeout) handler() {
    // Only this goroutine (and upstream handle_command() and restart()) can touch Timeout private data
loop:
    for {
        select {
        case cmd := <- timeout.control:
           if !timeout.handle_command(cmd) {
                break loop
            }
        case timeout.info <- TimeoutInfo{timeout.eta, timeout.alive}:
            continue
        }
    }
}

func (timeout *Timeout) handle_command(cmd TimeoutControl) (bool) {
    switch cmd.name {
    case "reset":
        timeout.avgto = cmd.avgto
        timeout.fudge = cmd.fudge
        timeout.restart()
    case "restart":
        timeout.restart()
    case "trigger":
        timeout.alive = false
        if timeout.cbch != nil {
            timeout.cbch <- Event{timeout.cbchmsg, timeout}
        }
    case "stop":
        timeout.impl.Stop()
        timeout.alive = false
    case "free":
        timeout.impl.Stop()
        timeout.alive = false
        timeout.cbch = nil
        // make sure program will panic if anybody tries to use this afterwards
        close(timeout.control)
        close(timeout.info)
        return false
    }
    return true
}

func (timeout *Timeout) restart() {
    if timeout.impl != nil {
        timeout.impl.Stop()
    }

    relative_eta := timeout.avgto + 2 * timeout.fudge * time.Duration(rand.Float32() - 0.5)
    timeout.eta = time.Now().Add(relative_eta)
    timeout.alive = true 

    timeout.impl = time.AfterFunc(relative_eta, func() {
        // goroutine context; make sure it goes through the control channel
        timeout.control <- TimeoutControl{"trigger", 0, 0}
    })
}

// public methods for Timeout

func (timeout *Timeout) Stop() {
    timeout.control <- TimeoutControl{"stop", 0, 0}
}

func (timeout *Timeout) Free() {
    // if called by the same goroutine that will call TCPSession.Close(), this guarantees that
    // there won't be a race between section destructor and user freeing the Timeout in parallel
    if timeout.cbchowner != nil {
        timeout.cbchowner.ReleaseTimeout(timeout)
        timeout.cbchowner = nil
    }
    // Synchronously guarantees that this Timeout won't post events after Free() returns
    // (possible because timeout.control is bufferless)
    timeout.control <- TimeoutControl{"free", 0, 0}
}

func (timeout *Timeout) Restart() {
    timeout.control <- TimeoutControl{"restart", 0, 0}
}

func (timeout *Timeout) Reset(avgto time.Duration, fudge time.Duration) {
    timeout.control <- TimeoutControl{"reset", avgto, fudge}
}

func (timeout *Timeout) Alive() (bool) {
    info := <- timeout.info
    return info.alive
}

func (timeout *Timeout) Remaining() (time.Duration) {
    info := <- timeout.info
    if !info.alive {
        return 0
    }
    return info.eta.Sub(time.Now()) 
}
