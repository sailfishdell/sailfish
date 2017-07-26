package provider

import (
	"context"
	"errors"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/domain"
	"net/http"
)

type RequestAdapter struct {
	Handle func(ctx context.Context, r *http.Request, privileges []string, cmdid eh.UUID, tree *domain.RedfishTree, item *domain.RedfishResource) error
}

type ProviderRegisterer interface {
	RegisterHandler(add string, adapter RequestAdapter)
	domain.DDDFunctions
}

// standard handler if we are just going to send the standard RemoveRedfishResource on delete
func MakeStandardHTTPDelete(d domain.DDDFunctions) func(context.Context, *http.Request, []string, eh.UUID, *domain.RedfishTree, *domain.RedfishResource) error {
	return func(ctx context.Context, r *http.Request, privileges []string, cmdID eh.UUID, tree *domain.RedfishTree, requested *domain.RedfishResource) error {
		idToDelete, ok := tree.Tree[r.URL.Path]
		if !ok {
			return errors.New("ID to delete doesn't appear to exist")
		}

		d.GetCommandBus().HandleCommand(ctx, &domain.RemoveRedfishResource{RedfishResourceAggregateBaseCommand: domain.RedfishResourceAggregateBaseCommand{UUID: idToDelete}})

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
