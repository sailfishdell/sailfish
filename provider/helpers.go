package provider

import (
	"context"
	"errors"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/domain"
	"net/http"
)

// standard handler if we are just going to send the standard RemoveRedfishResource on delete
func MakeStandardHTTPDelete(d domain.DDDFunctions) func(context.Context, eh.UUID, eh.UUID, *domain.RedfishResource, *http.Request) error {
	return func(ctx context.Context, treeID, cmdID eh.UUID, resource *domain.RedfishResource, r *http.Request) error {
		// we have the tree ID, fetch an updated copy of the actual tree
		// TODO: Locking? Should repo give us a copy? Need to test this.
		tree, err := domain.GetTree(ctx, d.GetReadRepo(), treeID)
		if err != nil {
			return err
		}

		sessionID, ok := tree.Tree[r.URL.Path]
		if !ok {
			return errors.New("Couldn't get handle for session service")
		}

		d.GetCommandBus().HandleCommand(ctx, &domain.RemoveRedfishResource{RedfishResourceAggregateBaseCommand: domain.RedfishResourceAggregateBaseCommand{UUID: sessionID}})

		event := eh.NewEvent(domain.HTTPCmdProcessedEvent,
			&domain.HTTPCmdProcessedData{
				CommandID: cmdID,
				Results:   map[string]interface{}{"msg": "complete!"},
				Headers:   map[string]string{},
			})

		d.GetEventHandler().HandleEvent(ctx, event)

		return nil
	}
}
