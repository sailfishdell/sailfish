package dell_ec

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func updateReset(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nUPDATE RESET\n\n")
	retData.Results = map[string]interface{}{"msg": "UPDATE RESET"}
	retData.StatusCode = 200
	return nil
}

func updateEID674Reset(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nUPDATE EID 674 RESET\n\n")
	retData.Results = map[string]interface{}{"msg": "UPDATE EID 674 RESET!"}
	retData.StatusCode = 200
	return nil
}
