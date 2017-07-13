package domain

import (
	"context"
	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/saga"
)

const CollectionSagaType saga.Type = "CollectionSaga"

type CollectionSaga struct {
	collectionList []eh.UUID
}

func NewCollectionSaga() *CollectionSaga {
	return &CollectionSaga{}
}

func SetupCollectionSaga() {
}

func (s *CollectionSaga) SagaType() saga.Type { return CollectionSagaType }

// implement automatic addition and removal from collections
func (s *CollectionSaga) RunSaga(ctx context.Context, event eh.Event) []eh.Command {
	switch event.EventType() {

	case RedfishResourceCreatedEvent:
		if _, ok := event.Data().(*RedfishResourceCreatedData); ok {
		}

	case RedfishResourceRemovedEvent:
	}

	return nil
}
