package domain

import (
	"context"
    "fmt"
	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/saga"
)

var _ = fmt.Printf

const PrivilegeSagaType saga.Type = "PrivilegeSaga"

type PrivilegeSaga struct {
}

func NewPrivilegeSaga() *PrivilegeSaga {
	return &PrivilegeSaga{}
}

func (s *PrivilegeSaga) SagaType() saga.Type { return PrivilegeSagaType }

func (s *PrivilegeSaga) RunSaga(ctx context.Context, event eh.Event) []eh.Command {
	switch event.EventType() {

	case OdataResourceCreatedEvent:
        fmt.Println("Adding privileges!")
		return []eh.Command{
			&UpdateOdataResourcePrivileges{
				UUID:       event.AggregateID(),
				Privileges: map[string]interface{}{"GET": []string{"Unauthenticated"}},
			},
		}
	}

	return nil
}
