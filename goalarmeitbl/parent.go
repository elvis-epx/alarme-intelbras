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

type ParentObserver interface {
    ChildDied(string, string, Child)
}

// Proxy object, to be added to real parents by composition or embedding
type Parent struct {
    observer ParentObserver
    parent_name string
    child_name string
    children map[ChildId]Child
    mutex sync.Mutex
}

func NewParent(parent_name string, child_name string, observer ParentObserver) *Parent {
    t := new(Parent)
    t.observer = observer
    t.parent_name = parent_name
    t.child_name = child_name
    t.children = make(map[ChildId]Child)
    return t
}

// called back by child
func (t *Parent) Adopt(child Child) {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    child_id := child.GetChildId()
    t.children[child_id] = child
    log.Printf("%s: Adopted %s %s", t.parent_name, t.child_name, child_id)
}

// called back by child
func (t *Parent) Died(child Child) {
    t.mutex.Lock()

    child_id := child.GetChildId()
    delete(t.children, child_id)
    log.Printf("%s: Died %s %s", t.parent_name, t.child_name, child_id)

    t.mutex.Unlock()

    if t.observer != nil {
        t.observer.ChildDied(t.parent_name, t.child_name, child)
    }
}

// called by real parent
func (t *Parent) DisownAll() {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    for child_id := range t.children {
        t.children[child_id].Disowned() // must not terminate siblings!
        log.Printf("%s: Disowned %s %s", t.parent_name, t.child_name, child_id)
    }
    t.children = make(map[ChildId]Child)
}
