package dell_ec

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

// TODO: need a logger
//    -- > request logger? <-- probably. get an example here

func bmcReset(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC RESET\n\n")
	// format message to the pump
	// set up waiter to wait for pump return message
	// send message to pump
	// wait for return
	// process the return
	// setup http return codes/results for 'retData'
	retData.Results = map[string]interface{}{"msg": "RESET!"}
	retData.StatusCode = 200
	return nil
}

func bmcResetToDefaults(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC RESET TO DEFAULTS\n\n")
	retData.Results = map[string]interface{}{"msg": "BMC RESET TO DEFAULTS!"}
	retData.StatusCode = 200
	return nil
}
func bmcForceFailover(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Force Failover\n\n")
	retData.Results = map[string]interface{}{"msg": "BMC Force Failover!"}
	retData.StatusCode = 200
	return nil
}
func exportSystemConfiguration(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Export System Configuration\n\n")
	retData.Results = map[string]interface{}{"msg": "BMC Export System Configuration!"}
	retData.StatusCode = 200
	return nil
}
func importSystemConfiguration(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Import System Configuration\n\n")
	retData.Results = map[string]interface{}{"msg": "BMC Import System Configuration!"}
	retData.StatusCode = 200
	return nil
}
func importSystemConfigurationPreview(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Import System Configuration Preview\n\n")
	retData.Results = map[string]interface{}{"msg": "BMC Import System Configuration Preview!"}
	retData.StatusCode = 200
	return nil
}
