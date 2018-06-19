package dell_ec

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func bmcReset(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC RESET\n\n")
	return nil
}

func bmcResetToDefaults(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC RESET TO DEFAULTS\n\n")
	return nil
}
func bmcForceFailover(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Force Failover\n\n")
	return nil
}
func exportSystemConfiguration(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Export System Configuration\n\n")
	return nil
}
func importSystemConfiguration(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Import System Configuration\n\n")
	return nil
}
func importSystemConfigurationPreview(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Import System Configuration Preview\n\n")
	return nil
}
