package bmc

import (
	"context"
	"fmt"

	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func init() {
	domain.RegisterInitFN(InitService)
}

// wait in a listener for the root service to be created, then extend it
func InitService(ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// background context to use
	ctx := context.Background()


    sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURI("/redfish/v1/Managers"))
    if err == nil  {
        sp.RunOnce( func(event eh.Event) {
		    NewService(ctx, ch)
        })
    }
}

func NewService(ctx context.Context, ch eh.CommandHandler) {
	// Create Computer System Collection
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: false,
			ResourceURI: "/redfish/v1/Managers/bmc",
			Type:        "#Manager.v1_1_0.Manager",
			Context:     "/redfish/v1/$metadata#Manager.Manager",
			Privileges: map[string]interface{}{
				"GET":    []string{"ConfigureManager"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
                "Id": "bmc",
                "Name": "Manager",
                "ManagerType": "BMC",
                "Description": "BMC",
                "ServiceEntryPointUUID": "92384634-2938-2342-8820-489239905423",
                "UUID": "00000000-0000-0000-0000-000000000000",
                "Model": "CatfishBMC",
                "DateTime": "2015-03-13T04:14:33+06:00",
                "DateTimeLocalOffset": "+06:00",
                "Status": {
                    "State": "Enabled",
                    "Health": "OK"
                },
                "FirmwareVersion": "1.00",
                "NetworkProtocol": {
                    "@odata.id": "/redfish/v1/Managers/bmc/NetworkProtocol"
                },
                "EthernetInterfaces": {
                    "@odata.id": "/redfish/v1/Managers/bmc/EthernetInterfaces"
                },
                "Links": {
                    "ManagerForServers": [
                        {
                            "@odata.id": "/redfish/v1/Systems/2M220100SL"
                        },
                        {
                            "@odata.id": "/redfish/v1/Systems/"
                        }
                    ],
                    "ManagerForChassis": [
                        {
                            "@odata.id": "/redfish/v1/Chassis/A33"
                        }
                    ],
                    "ManagerInChassis": {
                        "@odata.id": "/redfish/v1/Chassis/A33"
                    },
                    "Oem": {}
                },
                "Actions": {
                    "#Manager.Reset": {
                        "target": "/redfish/v1/Managers/bmc/Actions/Manager.Reset",
                        "ResetType@Redfish.AllowableValues": [
                            "ForceRestart",
                            "GracefulRestart"
                        ]
                    },
                    "Oem": {}
                },
                "Oem": {},
			}})
}
