package dell_ec

import (
        "context"
        "fmt"

        eh "github.com/looplab/eventhorizon"
        domain "github.com/superchalupa/go-redfish/src/redfishresource"
)


func iomResetPeakPowerConsumption(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nIOM RESET PEAK POWER CONSUMPTION\n\n")
	retData.Results = map[string]interface{}{"msg": "IOM RESET PEAK POWER CONSUMPTION!"}
	retData.StatusCode = 200
	return nil
}

func iomVirtualReseat(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nIOM VIRTUAL RESEAT\n\n")
	retData.Results = map[string]interface{}{"msg": "IOM VIRTUAL RESEAT!"}
	retData.StatusCode = 200
	return nil
}
