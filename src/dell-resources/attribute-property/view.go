package attribute_property

import (
	"context"
	"errors"
	"fmt"

	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func formatAttributeDump(
	ctx context.Context,
	v *view.View,
	m *model.Model,
	agg *domain.RedfishResourceAggregate,
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

	attributes, ok := v.GetModel("default").GetProperty(prop).(map[string]map[string]map[string]interface{})
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

func NewView(ctx context.Context, s *model.Model, c *ARDump) *view.View {
	v := view.NewView(
		view.MakeUUID(),
		view.WithModel(s),
		view.WithFormatter("attributeFormatter", formatAttributeDump),
		view.WithNamedController("ar_dump", c),
		view.WithUniqueName(fmt.Sprintf("%v", eh.NewUUID())),
	)

	domain.RegisterPlugin(func() domain.Plugin { return v })
	return v
}

func EnhanceExistingUUID(ctx context.Context, v *view.View, ch eh.CommandHandler, baseUUID eh.UUID) {
	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: baseUUID,
			Properties: map[string]interface{}{
				"Attributes@meta": v.Meta(view.GETProperty("attributes"), view.GETFormatter("attributeFormatter"), view.GETModel("default"), view.PropPATCH("attributes", "ar_dump")),
			},
		})
}
