package stdmeta

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"sync"

	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func RegisterFormatters(s *testaggregate.Service, d *domain.DomainObjects) {
	expandOneFormatter := MakeExpandOneFormatter(d)
	s.RegisterViewFunction("withFormatter_expandone", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding expandone formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("expandone", expandOneFormatter))

		return nil
	})

	expandFormatter := MakeExpandListFormatter(d)
	s.RegisterViewFunction("withFormatter_expand", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding expand formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("expand", expandFormatter))

		return nil
	})

	s.RegisterViewFunction("withFormatter_count", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding count formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("count", CountFormatter))

		return nil
	})

	s.RegisterViewFunction("withFormatter_attributeFormatter", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding attributeFormatter formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump))

		return nil
	})

	s.RegisterViewFunction("withFormatter_formatOdataList", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding FormatOdataList formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("formatOdataList", FormatOdataList))

		return nil
	})

	s.RegisterViewFunction("stdFormatters", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding standard formatters (expand, expandone, count, attributeFormatter, formatOdataList) to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("expandone", expandOneFormatter))
		vw.ApplyOption(view.WithFormatter("expand", expandFormatter))
		vw.ApplyOption(view.WithFormatter("count", CountFormatter))
		vw.ApplyOption(view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump))
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

		uris, ok := m.GetPropertyOk(p)
		if !ok {
			uris = []string{}
		}

		odata := []interface{}{}

		switch u := uris.(type) {
		case []string:
			for _, i := range u {
				out, err := d.ExpandURI(ctx, i)
				if err == nil {
					odata = append(odata, out)
				}
			}
		case []interface{}:
			for _, i := range u {
				j, ok := i.(string)
				if !ok {
					continue
				}
				out, err := d.ExpandURI(ctx, j)
				if err == nil {
					odata = append(odata, out)
				}
			}
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

		out, err := d.ExpandURI(ctx, uri)
		if err == nil {
			rrp.Value = out
		}

		return nil
	}
}

func CountFormatter(
	ctx context.Context,
	vw *view.View,
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

	v := reflect.ValueOf(arr)
	switch v.Kind() {
	case reflect.String:
		rrp.Value = v.Len()
	case reflect.Array:
		rrp.Value = v.Len()
	case reflect.Slice:
		rrp.Value = v.Len()
	case reflect.Map:
		rrp.Value = v.Len()
	case reflect.Chan:
		rrp.Value = v.Len()
	default:
		rrp.Value = 0
	}

	return nil
}

func FormatOdataList(ctx context.Context, v *view.View, m *model.Model, rrp *domain.RedfishResourceProperty, meta map[string]interface{}) error {
	p, ok := meta["property"].(string)

	uris, ok := m.GetPropertyOk(p)
	if !ok {
		uris = []string{}
	}

	sort.Strings(uris)

	odata := []interface{}{}
	switch u := uris.(type) {
	case []string:
		for _, i := range u {
			odata = append(odata, map[string]interface{}{"@odata.id": i})
		}
	case []interface{}:
		for _, i := range u {
			odata = append(odata, map[string]interface{}{"@odata.id": i})
		}
	}

	rrp.Value = odata

	return nil
}
