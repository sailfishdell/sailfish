package power

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func AddView(ctx context.Context, logger log.Logger, s *model.Service, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          model.GetUUID(s),
			Collection:  false,
			ResourceURI: model.GetOdataID(s),
			Type:        "#Power.v1_0_2.Power",
			Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":          "Power",
				"Description": "Power",
				"Name":        "Power",

                 // TODO: "PowerControl@odata.count": 1,
                 "PowerControl": [
                      {
                           "PowerLimit": {
                                "LimitInWatts": 0
                           },
                           "Name": "System Power Control",
                           "@odata.id": "/redfish/v1/Chassis/System.Chassis.1/Power/PowerControl",
                           "PowerConsumedWatts": 1000,
                           "MemberId": "PowerControl",
                           "PowerAvailableWatts": 6000,
                           "RelatedItem@odata.count": 41,
                           "PowerCapacityWatts": 0,
                           "Oem": {
                                "MaxPeakWatts": 1472,
                                "MinPeakWattsTime": "2018-04-04T20:33:28+0530",
                                "EnergyConsumptionkWh": 1,
                                "HeadroomWatts": 6000,
                                "MinPeakWatts": 983,
                                "PeakHeadroomWatts": 5528,
                                "MaxPeakWattsTime": "2018-04-04T19:03:59+0530",
                                "EnergyConsumptionStartTime": "1970-01-01T05:30:00+0530"
                           },
                           "RelatedItem": [],
                           "PowerMetrics": {
                                "MinConsumedWatts": 0,
                                "MaxConsumedWatts": 0,
                                "AverageConsumedWatts": 0,
                                "IntervalInMin": 0
                           }
                      }
                 ],


			}})
}
