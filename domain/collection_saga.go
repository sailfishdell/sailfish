package domain

import (
	"context"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/eventhandler/saga"

	"fmt"
)

const CollectionSagaType saga.Type = "CollectionSaga"

type CollectionSaga struct {
	collectionList []eh.UUID
	DDDFunctions
}

func NewCollectionSaga(d DDDFunctions) *CollectionSaga {
	return &CollectionSaga{DDDFunctions: d}
}

func SetupCollectionSaga(d DDDFunctions) {
	collectionSaga := saga.NewEventHandler(NewCollectionSaga(d), d.GetCommandBus())
	d.GetEventBus().AddHandler(collectionSaga, RedfishResourceCreatedEvent)
	d.GetEventBus().AddHandler(collectionSaga, RedfishResourceRemovedEvent)
}

func (s *CollectionSaga) SagaType() saga.Type { return CollectionSagaType }

// implement automatic addition and removal from collections
func (s *CollectionSaga) RunSaga(ctx context.Context, event eh.Event) []eh.Command {
	switch event.EventType() {
	case RedfishResourceCreatedEvent:
		fmt.Printf("COLLECTION SAGA: added\n")
		if _, ok := event.Data().(*RedfishResourceCreatedData); ok {
		}

	case RedfishResourceRemovedEvent:
		fmt.Printf("COLLECTION SAGA: removed\n")
	}

	return nil
}
