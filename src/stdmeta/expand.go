package stdmeta

import (
	"context"
	"errors"
	"path"
	"reflect"
	"sort"
	"strconv"
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
	domain.RegisterPlugin(func() domain.Plugin { return &uriCollection{d: d} })

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

	fastExpandFormatter := MakeFastExpandListFormatter(d)
	s.RegisterViewFunction("withFormatter_fastexpand", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		logger.Debug("Adding fast expand formatter to view", "view", vw.GetURI())
		vw.ApplyOption(view.WithFormatter("fastexpand", fastExpandFormatter))
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
		vw.ApplyOption(view.WithFormatter("fastexpand", fastExpandFormatter))
		vw.ApplyOption(view.WithFormatter("count", CountFormatter))
		vw.ApplyOption(view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump))
		vw.ApplyOption(view.WithFormatter("formatOdataList", FormatOdataList))
		return nil
	})
}

type uriCollection struct {
	d *domain.DomainObjects
}

var (
	uriCollectionPlugin = domain.PluginType("uricollection")
)

func (t *uriCollection) PluginType() domain.PluginType { return uriCollectionPlugin }

func (t *uriCollection) PropertyGet(
	ctx context.Context,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error {

	p, ok := meta["uribase"].(string)
	if !ok {
		return errors.New("uribase not specified as a string")
	}

	uriList := t.d.FindMatchingURIs(func(uri string) bool { return path.Dir(uri) == p })
	odata := make([]interface{}, len(uriList))

	sort.Slice(uriList, func(i, j int) bool {
		idx_i, _ := strconv.Atoi(path.Base(uriList[i]))
		idx_j, _ := strconv.Atoi(path.Base(uriList[j]))
		return idx_i > idx_j
	})

	used := 0
	for _, uri := range uriList {
		out, err := t.d.ExpandURI(ctx, uri)
		if err != nil {
			continue
		}
		odata[used] = out.Value
		used = used + 1
	}

	rrp.Value = odata[:used]

	return nil
}

func MakeFastExpandListFormatter(d *domain.DomainObjects) func(context.Context, *view.View, *model.Model, *domain.RedfishResourceProperty, *domain.RedfishAuthorizationProperty, map[string]interface{}) error {
	return func(
		ctx context.Context,
		v *view.View,
		m *model.Model,
		rrp *domain.RedfishResourceProperty,
		auth *domain.RedfishAuthorizationProperty,
		meta map[string]interface{},
	) error {

		u := uriCollection{d: d}
		u.PropertyGet(ctx, auth, rrp, meta)

		count, ok := meta["property"].(string)
		if ok {
			m.UpdateProperty(count, len(rrp.Value.([]interface{})))
		}

		return nil
	}
}

func MakeExpandListFormatter(d *domain.DomainObjects) func(context.Context, *view.View, *model.Model, *domain.RedfishResourceProperty, *domain.RedfishAuthorizationProperty, map[string]interface{}) error {
	return func(
		ctx context.Context,
		v *view.View,
		m *model.Model,
		rrp *domain.RedfishResourceProperty,
		auth *domain.RedfishAuthorizationProperty,
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
					odata = append(odata, out.Value)
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
					odata = append(odata, out.Value)
				}
			}
		}

		rrp.Value = odata

		return nil
	}
}

func MakeExpandOneFormatter(d *domain.DomainObjects) func(context.Context, *view.View, *model.Model, *domain.RedfishResourceProperty, *domain.RedfishAuthorizationProperty, map[string]interface{}) error {
	return func(
		ctx context.Context,
		v *view.View,
		m *model.Model,
		rrp *domain.RedfishResourceProperty,
		auth *domain.RedfishAuthorizationProperty,
		meta map[string]interface{},
	) error {
		p, ok := meta["property"].(string)

		uri, ok := m.GetProperty(p).(string)
		if !ok {
			return errors.New("uri property not setup properly")
		}

		out, err := d.ExpandURI(ctx, uri)
		if err == nil {
			rrp.Value = out.Value
		}

		return nil
	}
}

func CountFormatter(
	ctx context.Context,
	vw *view.View,
	m *model.Model,
	rrp *domain.RedfishResourceProperty,
	auth *domain.RedfishAuthorizationProperty,
	meta map[string]interface{},
) error {
	p, ok := meta["property"].(string)
	if !ok {
		return errors.New("property name to operate on not passed in meta.")
	}

	arr := m.GetProperty(p)
	if arr == nil {
		rrp.Value = 0
		return errors.New("array property not setup properly, however setting count to 0")
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

func FormatOdataList(ctx context.Context, v *view.View, m *model.Model, rrp *domain.RedfishResourceProperty, auth *domain.RedfishAuthorizationProperty, meta map[string]interface{}) error {
	p, ok := meta["property"].(string)

	uris, ok := m.GetPropertyOk(p)
	if !ok {
		uris = []string{}
	}

	var uriArr []string
	odata := []interface{}{}

	switch u := uris.(type) {
	case []string:
		uriArr = u
	case []interface{}:
		uriArr = []string{}
		for _, i := range u {
			if s, ok := i.(string); ok {
				uriArr = append(uriArr, s)
			}
		}
	}

	sort.Strings(uriArr)
	for _, i := range uriArr {
		odata = append(odata, map[string]interface{}{"@odata.id": i})
	}

	rrp.Value = odata

	return nil
}
