package dell_ec

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func chassisReset(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nCHASSIS RESET\n\n")
	retData.Results = map[string]interface{}{"msg": "CHASSIS RESET!"}
	retData.StatusCode = 200
	return nil
}

func msmConfigBackup(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nMSM CONFIG BACKUP\n\n")
	retData.Results = map[string]interface{}{"msg": "MSM CONFIG BACKUP!"}
	retData.StatusCode = 200
	return nil
}

func chassisMSMConfigBackup(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nCHASSIS MSM CONFIG BACKUP\n\n")
	retData.Results = map[string]interface{}{"msg": "CHASSIS MSM CONFIG BACKUP!"}
	retData.StatusCode = 200
	return nil
}
