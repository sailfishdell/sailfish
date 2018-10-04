package dell_ec

import (
	"context"
	"errors"

	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func makeExpandFormatter(d *domain.DomainObjects) func(context.Context, *view.View, *model.Model, *domain.RedfishResourceProperty, map[string]interface{}) error {
	return func(
		ctx context.Context,
		v *view.View,
		m *model.Model,
		rrp *domain.RedfishResourceProperty,
		meta map[string]interface{},
	) error {
		p, ok := meta["property"].(string)

		uris, ok := m.GetProperty(p).([]string)
		if !ok {
			return errors.New("uris property not setup properly")
		}

		odata := []map[string]interface{}{}
		for _, i := range uris {

			aggID, ok := d.GetAggregateIDOK(i)
			if !ok {
				continue
			}
			agg, _ := d.AggregateStore.Load(ctx, domain.AggregateType, aggID)
			redfishResource, ok := agg.(*domain.RedfishResourceAggregate)
			if !ok {
				continue
			}

			redfishResource.PropertiesMu.RLock()
			sub, _ := domain.ProcessGET(ctx, redfishResource.Properties)
			redfishResource.PropertiesMu.RUnlock()

			odata = append(odata, map[string]interface{}{"@odata.id": sub})
		}

		rrp.Value = odata

		return nil
	}
}

func countFormatter(
	ctx context.Context,
	v *view.View,
	m *model.Model,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error {
	p, ok := meta["property"].(string)

	uris, ok := m.GetProperty(p).([]string)
	if !ok {
		return errors.New("uris property not setup properly")
	}

	rrp.Value = len(uris)

	return nil
}
