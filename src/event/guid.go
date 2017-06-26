package eventsourcing

import "github.com/twinj/uuid"

// An item having a GUID
type GUIDer interface {
	GetGUID() guid
	SetGUID(guid)
}

// Base implementation for all Guiders
type withGUID struct {
	GUID guid
}

func (e *withGUID) SetGUID(g guid) {
	e.GUID = g
}
func (e *withGUID) GetGUID() guid {
	return e.GUID
}

type guid string

// Create a new GUID - use UUID v4
func newGUID() guid {
	return guid(uuid.NewV4().String())
}
