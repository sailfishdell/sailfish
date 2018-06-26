package dell_ec                                                                                                                        

import (
        "context"
        "fmt"

        eh "github.com/looplab/eventhorizon"
        domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func submitTestEvent(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nSUBMIT TEST EVENT\n\n")
	retData.Results = map[string]interface{}{"msg": "SUBMIT TEST EVENT!"}
	retData.StatusCode = 200
	return nil
}
