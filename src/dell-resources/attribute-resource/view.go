package attribute_resource

import (
	"context"

	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func (s *service) AddView(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          model.GetUUID(s),
			Collection:  false,
			ResourceURI: model.GetOdataID(s),
			Type:        "#OemAttributes.v1_0_0.OemAttributes",
			Context:     "/redfish/v1/$metadata#OemAttributes.OemAttributes",

			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":                s.GetProperty("unique_name"),
				"Name":              "Oem Attributes",
				"Description":       "This is the manufacturer/provider specific list of attributes.",
				"AttributeRegistry": "ManagerAttributeRegistry.v1_0_0",
			}})
}
