package computersystem

import (
	"context"
	"encoding/json"
	"errors"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/domain"
	"github.com/superchalupa/go-redfish/provider"
	"net/http"

	"fmt"
)

// so for actions, we have choices
//  - We can re-use a POST to the object itself so that we inherit the
//    privileges from the base object.
//  - We can create a new RedfishResource aggregate object that will be a
//    container to hold the privileges for the actions

func SetupActions(s provider.ProviderRegisterer) {
	s.RegisterHandler(
		// here we re-use the base computer system object because POST isn't
		// otherwise used. The action object will have to specify the id
		"POST:type:#ComputerSystem.v1_1_0.ComputerSystem", // ?system=437XR1138R2,
		provider.RequestAdapter{Handle: makeHandlePost(s)},
	)
	return
}

func makeHandlePost(d domain.DDDFunctions) func(context.Context, *http.Request, []string, eh.UUID, *domain.RedfishTree, *domain.RedfishResource) error {
	return func(ctx context.Context, r *http.Request, privileges []string, cmdID eh.UUID, tree *domain.RedfishTree, requested *domain.RedfishResource) error {

		// ParseForm eats r.Body
		r.ParseForm()
		fmt.Printf("form values: %#v\n", r.Form)
		fmt.Printf("postform values: %#v\n", r.PostForm)

		action, ok := r.Form["action"]
		if !ok {
			return errors.New("action not specified")
		}

		// might be able to relax this later, but only one action can have data if we do
		if len(action) != 1 {
			return errors.New("please specify one and only one action")
		}

		if action[0] != "ComputerSystem.Reset" {
			return errors.New("invalid action: " + action[0])
		}

		decoder := json.NewDecoder(r.Body)
		var rr provider.ComputerSystemResetRequestData
		err := decoder.Decode(&rr)

		if err != nil {
			return err
		}

		rr.CorrelationID = cmdID

		// Validate reset request
		validResetTypes := []string{
			"On",
			"ForceOff",
			"GracefulShutdown",
			"GracefulRestart",
			"ForceRestart",
			"Nmi",
			"ForceOn",
			"PushPowerButton",
			"PowerCycle",
		}

		for _, i := range validResetTypes {
			if rr.ResetType == i {
				goto valid
			}
		}

		return errors.New(fmt.Sprintf("Invalid reset type(%v), not in list: %v", rr.ResetType, validResetTypes))

	valid:
		// send out the request for reset
		d.GetEventHandler().HandleEvent(ctx, eh.NewEvent(provider.ComputerSystemResetRequestEvent, &rr))

		// send out the http response
		event := eh.NewEvent(domain.HTTPCmdProcessedEvent,
			&domain.HTTPCmdProcessedData{
				CommandID: cmdID,
				Results:   map[string]interface{}{"result": "success"},
			})

		d.GetEventHandler().HandleEvent(ctx, event)
		return nil
	}
}
