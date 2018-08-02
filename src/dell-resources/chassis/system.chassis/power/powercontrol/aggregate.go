package powercontrol

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) map[string]interface{} {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#Power.v1_0_2.PowerControl",
			Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: getViewFragment(v),
		})

	return getViewFragment(v)
}

func getViewFragment(v *view.View) map[string]interface{} {
	properties :=  map[string]interface{}{
		"@odata.type":             "#Power.v1_0_2.PowerControl",
		"@odata.context":          "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
		"@odata.id":               v.GetURI(),
		"Name@meta":               v.Meta(view.PropGET("name")),
		"MemberId@meta":           v.Meta(view.PropGET("unique_id")),
                "PowerAvailableWatts@meta": v.Meta(view.PropGET("available_watts")),
		"PowerCapacityWatts@meta": v.Meta(view.PropGET("capacity_watts")),
                "PowerConsumedWatts@meta": v.Meta(view.PropGET("consumed_watts")),

		"Oem": map[string]interface{}{
		    "EnergyConsumptionStartTime": null,
		    "EnergyConsumptionkWh": null,
		    "HeadroomWatts": null,
		    "MaxPeakWatts": null,
		    "MaxPeakWattsTime": null,
		    "MinPeakWatts": null,
		    "MinPeakWattsTime": null,
		    "PeakHeadroomWatts": null,
		},
		"PowerLimit": map[string]interface{}{
		    "LimitInWatts": "",
		},
		"PowerMetrics": map[string]interface{}{
		    "AverageConsumedWatts": 0,
		    "IntervalInMin": 0,
		    "MaxConsumedWatts": 0,
		    "MinConsumedWatts": 0,
		},
		"RelatedItem@meta": v.Meta(view.PropGET("related_item")),
		"RelatedItem@odata.count@meta": v.Meta(view.PropGET(related_item_count)), 
	}

	return properties
}
