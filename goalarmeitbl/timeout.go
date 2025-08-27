package goalarmeitbl

import (
    "time"
    "math/rand/v2"
    "log"
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
    avgto time.Duration
    fudge time.Duration
    impl *time.Timer
    alive bool
    eta time.Time

    cbch chan Event
    cbchmsg string
    owner *TimeoutOwner

    control chan TimeoutControl // must be bufferless, see Free() and "free" event
    info chan TimeoutInfo
}

type TimeoutCallback func (*Timeout)

func NewTimeout(avgto time.Duration, fudge time.Duration, cbch chan Event, cbchmsg string, owner *TimeoutOwner) (*Timeout) {
    timeout := Timeout{avgto, fudge, nil, false, time.Now(),
                cbch, cbchmsg, owner,
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
    if timeout.owner != nil {
        timeout.owner.release_timeout(timeout)
        timeout.owner = nil
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

// Timeout owner that is part of by e.g. TCPSession and TCPServer

type TimeoutOwner struct {
    cbch chan Event
    timeouts map[*Timeout]bool  // Timeouts associated with this server
    timeouts_sem chan struct {} // and its semaphore
}

func NewTimeoutOwner(cbch chan Event) *TimeoutOwner {
    t := new(TimeoutOwner)
    t.cbch = cbch
    t.timeouts = make(map[*Timeout]bool)
    t.timeouts_sem = make(chan struct{}, 1)
    t.timeouts_sem <-struct{}{}
    return t
}

// This is called by the owned Timeout upon Timeout.Free()
func (t *TimeoutOwner) release_timeout(to *Timeout) {
    <-t.timeouts_sem
    if _, ok := t.timeouts[to]; ok {
        log.Printf("Released timeout %p", to)
        delete(t.timeouts, to)
    }
    t.timeouts_sem <-struct{}{}
}

// Release all owned Timeouts and make sure they won't send further events
func (t *TimeoutOwner) Release() {
    for {
        var to *Timeout

        // Get some owned timeout
        <-t.timeouts_sem
	    for k := range t.timeouts {
		    to = k
		    break
	    }
        t.timeouts_sem <-struct{}{}

        if to == nil {
            break
        }

        // synchronously calls TimeoutOwner.release_timeout()
        to.Free()
    }
}

// Create new owned Timeout
func (t *TimeoutOwner) Timeout(avgto time.Duration, fudge time.Duration, cbchmsg string) (*Timeout) {
    <-t.timeouts_sem
    to := NewTimeout(avgto, fudge, t.cbch, cbchmsg, t)
    t.timeouts[to] = true
    t.timeouts_sem <-struct{}{}

    return to
}
