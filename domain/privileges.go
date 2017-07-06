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
    repo    eh.ReadRepo
}

func NewPrivilegeSaga(odataRepo eh.ReadRepo) *PrivilegeSaga {
	return &PrivilegeSaga{repo: odataRepo}
}

func (s *PrivilegeSaga) SagaType() saga.Type { return PrivilegeSagaType }

// RunSaga implements the Privilege handling. The final implementation of this will take the input, look up the entity in a privilegemap and emit commands to set the privileges on each new odataresource as it is created.
func (s *PrivilegeSaga) RunSaga(ctx context.Context, event eh.Event) []eh.Command {
	switch event.EventType() {

	case OdataResourceCreatedEvent:
        fmt.Println("Adding privileges!")

        // look up and set "Unauthenticated" for /redfish/ and /redfish/v1/ and session login link.
        // FOR NOW, set Login for all others. This needs to be fleshed out more
		if data, ok := event.Data().(*OdataResourceCreatedData); ok {
			ResourceURI := data.ResourceURI
            if ResourceURI == "/redfish/" || ResourceURI == "/redfish/v1/" {
                return []eh.Command{
                    &UpdateOdataResourcePrivileges{
                        UUID:       event.AggregateID(),
                        Privileges: map[string]interface{}{"GET": []string{"Unauthenticated"}},
                    },
                }
            } else {
                return []eh.Command{
                    &UpdateOdataResourcePrivileges{
                        UUID:       event.AggregateID(),
                        Privileges: map[string]interface{}{"GET": []string{"ConfigManager"}},
                    },
                }

            }
		}

        // Strategy for ConfigureSelf. If the Privilege Map specifies ConfigureSelf as privilege, then set the actual privilege to ConfigureSelf_%{USERNAME}, where %{USERNAME} is the username property of that resource

        // On login, set ConfigureSelf_${USERNAME} as a privilege based on username property

	}

	return nil
}
