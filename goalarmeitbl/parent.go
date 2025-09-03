package goalarmeitbl

import (
    "log"
    "sync"
)


type ChildId string

// Definition of what is a child
type Child interface {
    Disowned()
    GetChildId() ChildId
}

// Proxy object, to be added to real parents by composition or embedding
type Parent struct {
    children map[ChildId]Child
    mutex sync.Mutex
}

func NewParent() *Parent {
    t := new(Parent)
    t.children = make(map[ChildId]Child)
    return t
}

// called back by child
func (t *Parent) Adopt(child Child) {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    child_id := child.GetChildId()
    t.children[child_id] = child
    log.Printf("Parent %p: Adopted %s", t, child_id)
}

// called back by child
func (t *Parent) Disown(child Child) {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    child_id := child.GetChildId()
    delete(t.children, child_id)
    log.Printf("Parent %p: Disowned %s", t, child_id)
}

// called by real parent
func (t *Parent) DisownAll() {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    for child_id := range t.children {
        t.children[child_id].Disowned()
        log.Printf("Parent %p: Released %s", t, child_id)
    }
    t.children = make(map[ChildId]Child)
}
