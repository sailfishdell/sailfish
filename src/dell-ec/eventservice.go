package dell_ec

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func submitTestEvent(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nsubmit test event\n\n")
	// format message to the pump
	// set up waiter to wait for pump return message
	// send message to pump
	// wait for return
	// process the return
	// setup http return codes/results for 'retData'
	retData.Results = map[string]interface{}{"msg": "submitted (NOT!)"}
	retData.StatusCode = 500
	return nil
}
