package dell_ec

import (
	"context"
	"errors"
	"reflect"

	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func RegisterFormatters(d *domain.DomainObjects) {
	expandOneFormatter := MakeExpandOneFormatter(d)
	testaggregate.RegisterViewFunction("withFormatter_expandone", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding expandone formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("expandone", expandOneFormatter))

		return nil
	})

	expandFormatter := MakeExpandListFormatter(d)
	testaggregate.RegisterViewFunction("withFormatter_expand", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding expand formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("expand", expandFormatter))

		return nil
	})

	testaggregate.RegisterViewFunction("withFormatter_count", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding count formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("count", CountFormatter))

		return nil
	})

	testaggregate.RegisterViewFunction("withFormatter_attributeFormatter", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding attributeFormatter formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump))

		return nil
	})

	testaggregate.RegisterViewFunction("withFormatter_formatOdataList", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding FormatOdataList formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("formatOdataList", FormatOdataList))

		return nil
	})

}

func MakeExpandListFormatter(d *domain.DomainObjects) func(context.Context, *view.View, *model.Model, *domain.RedfishResourceProperty, map[string]interface{}) error {
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

		odata := []interface{}{}
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

			odata = append(odata, sub)
		}

		rrp.Value = odata

		return nil
	}
}

func MakeExpandOneFormatter(d *domain.DomainObjects) func(context.Context, *view.View, *model.Model, *domain.RedfishResourceProperty, map[string]interface{}) error {
	return func(
		ctx context.Context,
		v *view.View,
		m *model.Model,
		rrp *domain.RedfishResourceProperty,
		meta map[string]interface{},
	) error {
		p, ok := meta["property"].(string)

		uri, ok := m.GetProperty(p).(string)
		if !ok {
			return errors.New("uri property not setup properly")
		}

		aggID, ok := d.GetAggregateIDOK(uri)
		if !ok {
			return errors.New("could not find aggregate")
		}
		agg, _ := d.AggregateStore.Load(ctx, domain.AggregateType, aggID)
		redfishResource, ok := agg.(*domain.RedfishResourceAggregate)
		if !ok {
			return errors.New("specified uri wasn't an aggregate (impossible!)")
		}

		redfishResource.PropertiesMu.RLock()
		rrp.Value, _ = domain.ProcessGET(ctx, redfishResource.Properties)
		redfishResource.PropertiesMu.RUnlock()

		return nil
	}
}

func CountFormatter(
	ctx context.Context,
	v *view.View,
	m *model.Model,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error {
	p, ok := meta["property"].(string)
	if !ok {
		return errors.New("property name to operate on not passed in meta.")
	}

	arr := m.GetProperty(p)
	if arr == nil {
		return errors.New("array property not setup properly")
	}

	r := reflect.ValueOf(arr)
	rrp.Value = r.Len()

	return nil
}

func FormatOdataList(ctx context.Context, v *view.View, m *model.Model, rrp *domain.RedfishResourceProperty, meta map[string]interface{}) error {
	p, ok := meta["property"].(string)

	uris, ok := m.GetProperty(p).([]string)
	if !ok {
		return errors.New("uris property not setup properly")
	}

	odata := []interface{}{}
	for _, i := range uris {
		odata = append(odata, map[string]interface{}{"@odata.id": i})
	}

	rrp.Value = odata

	return nil
}
