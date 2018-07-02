package dell_ec

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func chassisPeripheralMapping(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nCHASSIS PERIPHERAL MAPPING\n\n")
	retData.Results = map[string]interface{}{"msg": "CHASSIS PERIPHERAL MAPPING!"}
	retData.StatusCode = 200
	return nil
}

func sledVirtualReseat(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nSLED VIRTUAL RESEAT\n\n")
	retData.Results = map[string]interface{}{"msg": "SLED VIRTUAL RESEAT!"}
	retData.StatusCode = 200
	return nil
}

func chassisSledVirtualReseat(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nCHASSIS SLED VIRTUAL RESEAT\n\n")
	retData.Results = map[string]interface{}{"msg": "CHASSIS SLED VIRTUAL RESEAT!"}
	retData.StatusCode = 200
	return nil
}
