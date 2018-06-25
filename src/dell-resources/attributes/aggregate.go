package attributes

import (
	"context"
	"errors"

	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func FormatAttributeDump(
	ctx context.Context,
	v *view.View,
	m *model.Model,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error {
	p, ok := meta["property"]
	if !ok {
		return errors.New("fallback")
	}

	prop, ok := p.(string)
	if !ok {
		return errors.New("fallback")
	}

	m.Lock()
	defer m.Unlock()
	attributes, ok := m.GetPropertyUnlocked(prop).(map[string]map[string]map[string]interface{})
	if !ok {
		return errors.New("attributes not setup properly")
	}

	res := map[string]interface{}{}
	for group, v := range attributes {
		for index, v2 := range v {
			for name, value := range v2 {
				res[group+"."+index+"."+name] = value
			}
		}
	}
	rrp.Value = res

	return nil
}

func EnhanceAggregate(ctx context.Context, v *view.View, ch eh.CommandHandler, baseUUID eh.UUID) {
	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: baseUUID,
			Properties: map[string]interface{}{
				"Attributes@meta": v.Meta(view.GETProperty("attributes"), view.GETFormatter("attributeFormatter"), view.GETModel("default"), view.PropPATCH("attributes", "ar_dump")),
			},
		})
}

func AddAggregate(ctx context.Context, v *view.View, uri string, ch eh.CommandHandler) (ret eh.UUID) {
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
				"Id":                v.Meta(view.GETProperty("unique_name"), view.GETModel("default")),
				"Name":              "Oem Attributes",
				"Description":       "This is the manufacturer/provider specific list of attributes.",
				"AttributeRegistry": "ManagerAttributeRegistry.v1_0_0",
				"Attributes@meta":   v.Meta(view.GETProperty("attributes"), view.GETFormatter("attributeFormatter"), view.GETModel("default"), view.PropPATCH("attributes", "ar_dump")),
			}})

	return
}
