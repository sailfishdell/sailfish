package redfishserver

import (
	"context"
	"sync"
)

/*
   tree node
       serialize(query/filter/select)
       Add node pointer
       delete node pointer
*/
type OdataSerializable interface {
	OdataSerialize(context.Context) (map[string]interface{}, error)
}

//     map[ URI ] JSON DATA
type OdataTree struct {
	items map[string]OdataSerializable
	sync.RWMutex
}

func NewOdataTree() OdataTree {
	return OdataTree{items: make(map[string]OdataSerializable)}
}

func (t OdataTree) Get(key string) (OdataSerializable, bool) {
	t.RLock()
	v, ok := t.items[key]
	t.RUnlock()
	return v, ok
}

func (t OdataTree) Set(key string, val OdataSerializable) {
	t.Lock()
	t.items[key] = val
	t.Unlock()
}
