// Copyright (c) 2017 - The Event Horizon authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aggregatestore

import (
	"context"
	"errors"
	//"fmt"

	eh "github.com/looplab/eventhorizon"
)

// ErrInvalidRepo is when a dispatcher is created with a nil repo.
var ErrInvalidRepo = errors.New("invalid repo")

// ErrInvalidAggregate occurs when a loaded aggregate is not an aggregate.
var ErrInvalidAggregate = errors.New("invalid aggregate")

// AggregateStore is an aggregate store that uses a read write repo for
// loading and saving aggregates.
type AggregateStore struct {
	repo eh.ReadWriteRepo
	bus  eh.EventBus
}

// NewAggregateStore creates an aggregate store with a read write repo.
func NewAggregateStore(repo eh.ReadWriteRepo, bus eh.EventBus) (*AggregateStore, error) {
	if repo == nil {
		return nil, ErrInvalidRepo
	}

	d := &AggregateStore{
		repo: repo,
		bus:  bus,
	}
	return d, nil
}

// Load implements the Load method of the eventhorizon.AggregateStore interface.
func (r *AggregateStore) Load(ctx context.Context, aggregateType eh.AggregateType, id eh.UUID) (eh.Aggregate, error) {
	item, err := r.repo.Find(ctx, id)
	if rrErr, ok := err.(eh.RepoError); ok && rrErr.Err == eh.ErrEntityNotFound {
		// Create the aggregate.
		if item, err = eh.CreateAggregate(aggregateType, id); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	aggregate, ok := item.(eh.Aggregate)
	if !ok {
		return nil, ErrInvalidAggregate
	}

	return aggregate, nil
}

type aLock interface {
	Lock()
	Unlock()
}

// Save implements the Save method of the eventhorizon.AggregateStore interface.
func (r *AggregateStore) Save(ctx context.Context, aggregate eh.Aggregate) error {
	var events = []eh.Event{} 
	publisher, ok := aggregate.(EventPublisher)
	if ok && r.bus != nil{
               events = publisher.EventsToPublish()
	}
		
       al, alok := aggregate.(aLock)
       if alok {
               //fmt.Println("lock successful")
               al.Lock()
       }

	err := r.repo.Save(ctx, aggregate)

       if alok {
               //fmt.Println("unlock successful")
               al.Unlock()
       }
	if err != nil {
		return err
	}

       // Publish events if supported by the aggregate.
	if ok && r.bus != nil {
 		publisher.ClearEvents()
               for _, e := range events {
                       r.bus.PublishEvent(ctx, e)
               }
       }

		
	return nil 
}

// Save implements the Save method of the eventhorizon.AggregateStore interface.
func (r *AggregateStore) Remove(ctx context.Context, aggregate eh.Aggregate) error {
	return r.repo.Remove(ctx, aggregate.EntityID())
}
