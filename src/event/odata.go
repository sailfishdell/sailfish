package eventsourcing

import ()

// An aggregate implementation representing odata
type Odata struct {
	baseAggregate
	withSequence
	properties map[string]interface{}
}

// Make sure it implements Aggregate
var _ Aggregate = (*Odata)(nil)
