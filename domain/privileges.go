package domain

import (
	"context"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/eventhandler/saga"
	"strings"
)

const PrivilegeSagaType saga.Type = "PrivilegeSaga"

type PrivilegeSaga struct {
	repo eh.ReadRepo
	// TODO: fix hardcoded /redfish references here
	redfishStartURI string
}

func NewPrivilegeSaga(redfishRepo eh.ReadRepo) *PrivilegeSaga {
	return &PrivilegeSaga{repo: redfishRepo}
}

func (s *PrivilegeSaga) SagaType() saga.Type { return PrivilegeSagaType }

// RunSaga implements the Privilege handling. The final implementation of this
// will take the input invents (RedfishResourceCreated), look up the entity in a
// privilegemap and emit commands to set the privileges on each new
// redfishresource as it is created. For now, just implementing basic stuff so we
// can demonstrate that it works.
func (s *PrivilegeSaga) RunSaga(ctx context.Context, event eh.Event) []eh.Command {
	switch event.EventType() {

	case RedfishResourceCreatedEvent:
		// look up and set "Unauthenticated" for /redfish/ and /redfish/v1/ and session login link.
		// FOR NOW, set Login for all others. This needs to be fleshed out more
		if data, ok := event.Data().(*RedfishResourceCreatedData); ok {
			ResourceURI := data.ResourceURI
			if ResourceURI == "/redfish/" || ResourceURI == "/redfish/v1/" {
				return []eh.Command{
					&UpdateRedfishResourcePrivileges{
						UUID:       event.AggregateID(),
						Privileges: map[string]interface{}{"GET": []string{"Unauthenticated"}},
					},
				}
			} else if ResourceURI == "/redfish/v1/SessionService/Sessions" {
				return []eh.Command{
					&UpdateRedfishResourcePrivileges{
						UUID: event.AggregateID(),
						Privileges: map[string]interface{}{
							"GET":    []string{"ConfigureManager"},
							"POST":   []string{"Unauthenticated"},
							"PUT":    []string{"ConfigureManager"},
							"PATCH":  []string{"ConfigureManager"},
							"DELETE": []string{"ConfigureSelf"},
						},
					},
				}

			} else if strings.HasPrefix(ResourceURI, "/redfish/v1/SessionService/Sessions/") {
				return []eh.Command{
					&UpdateRedfishResourcePrivileges{
						UUID: event.AggregateID(),
						Privileges: map[string]interface{}{
							"GET":    []string{"ConfigureManager"},
							"POST":   []string{"ConfigureManager"},
							"PUT":    []string{"ConfigureManager"},
							"PATCH":  []string{"ConfigureManager"},
							"DELETE": []string{"ConfigureSelf", "ConfigureManager"},
						},
					},
				}

			} else {
				return []eh.Command{
					&UpdateRedfishResourcePrivileges{
						UUID: event.AggregateID(),
						Privileges: map[string]interface{}{
							"GET":    []string{"ConfigureManager"},
							"POST":   []string{"ConfigureManager"},
							"PUT":    []string{"ConfigureManager"},
							"PATCH":  []string{"ConfigureManager"},
							"DELETE": []string{},
						},
					},
				}

			}
		}

		// Strategy for ConfigureSelf. If the Privilege Map specifies
		// ConfigureSelf as privilege, then set the actual privilege to
		// ConfigureSelf_%{USERNAME}, where %{USERNAME} is the username
		// property of that resource
		//
		// On the other end (login), set ConfigureSelf_${USERNAME} as a
		// privilege based on username property
	}

	return nil
}
