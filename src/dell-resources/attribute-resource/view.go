package attribute_resource

import (
	"context"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func AddView(ctx context.Context, uri, id string, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) (ret eh.UUID) {
	ret = eh.NewUUID()

	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          ret,
			Collection:  false,
			ResourceURI: uri,
			Type:        "#OemAttributes.v1_0_0.OemAttributes",
			Context:     "/redfish/v1/$metadata#OemAttributes.OemAttributes",

			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":                id,
				"Name":              "Oem Attributes",
				"Description":       "This is the manufacturer/provider specific list of attributes.",
				"AttributeRegistry": "ManagerAttributeRegistry.v1_0_0",
			}})

	return
}
