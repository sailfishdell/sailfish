package test_aggregate

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"

	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, v *view.View, ch eh.CommandHandler) *view.View {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#Manager.v1_0_2.Manager",
			Context:     "/redfish/v1/$metadata#Manager.Manager",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id@meta":          v.Meta(view.PropGET("unique_name")),
				"Name@meta":        v.Meta(view.PropGET("name"), view.PropPATCH("name", "ar_mapper")),
				"Description@meta": v.Meta(view.PropGET("description")),
				"Model@meta":       v.Meta(view.PropGET("model"), view.PropPATCH("model", "ar_mapper")),
			}})

	return v
}
