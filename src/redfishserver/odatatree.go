package redfishserver

import (
	"context"
	"sync"
)

/*
   tree node
       serialize(query/filter/select)
*/
type OdataSerializable interface {
	OdataSerialize(context.Context) (map[string]interface{}, error)
}

//     map[ URI ] JSON DATA
type OdataTree struct {
	bodies  map[string]OdataSerializable
	headers map[string]OdataSerializable
	sync.RWMutex
}

func NewOdataTree() OdataTree {
	return OdataTree{bodies: make(map[string]OdataSerializable)}
}

func (t OdataTree) GetBody(key string) (OdataSerializable, bool) {
	t.RLock()
	v, ok := t.bodies[key]
	t.RUnlock()
	return v, ok
}

func (t OdataTree) SetBody(key string, val OdataSerializable) {
	t.Lock()
	t.bodies[key] = val
	t.Unlock()
}

func (t OdataTree) GetHeaders(key string) (OdataSerializable, bool) {
	t.RLock()
	v, ok := t.headers[key]
	t.RUnlock()
	return v, ok
}

func (t OdataTree) SetHeaders(key string, val OdataSerializable) {
	t.Lock()
	t.headers[key] = val
	t.Unlock()
}
