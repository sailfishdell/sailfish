package domain

import (
	"context"
	"errors"
	"reflect"
	"sync"
)

type processFn func(context.Context, *RedfishResourceProperty, encOpts) (interface{}, error)

type encOpts struct {
	request   interface{}
	present   bool
	process   processFn
	fnArgAuth *RedfishAuthorizationProperty
}

func ProcessPATCH(ctx context.Context, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, request interface{}) (results interface{}, err error) {
	opts := encOpts{
		request:   request,
		process:   PATCHfn,
		fnArgAuth: auth,
	}

	val, err := parseRecursive(ctx, reflect.ValueOf(rrp), opts)
	if val.IsValid() {
		return val.Interface(), err
	}
	return nil, err
}

func ProcessGET(ctx context.Context, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty) (results interface{}, err error) {
	opts := encOpts{
		request:   nil,
		process:   GETfn,
		fnArgAuth: auth,
	}

	val, err := parseRecursive(ctx, reflect.ValueOf(rrp), opts)
	if val.IsValid() {
		return val.Interface(), err
	}
	return nil, err
}

func (rrp *RedfishResourceProperty) DOMETA(ctx context.Context, e encOpts) (results reflect.Value, err error) {
	//fmt.Printf("DOMETA\n")
	res, err := e.process(ctx, rrp, e)
  res2, err2 := parseRecursive(ctx, reflect.ValueOf(res), e)
  if _, ok := err.(IsHTTPCode); ok {
    return res2, err
  }
	return res2, err2
}

type Marshaler interface {
	DOMETA(context.Context, encOpts) (reflect.Value, error)
}

type Locker interface {
	Lock()
	Unlock()
}

var marshalerType = reflect.TypeOf(new(Marshaler)).Elem()
var locktype = reflect.TypeOf(new(Locker)).Elem()

func parseRecursive(ctx context.Context, val reflect.Value, e encOpts) (reflect.Value, error) {
	if !val.IsValid() {
		//fmt.Printf("NOT VALID, returning\n")
    return val, nil
	}

	if val.Type().Implements(locktype) {
		m := val.Interface().(Locker)
		m.Lock()
		defer m.Unlock()
	}

	if val.Type().Implements(marshalerType) {
		//fmt.Printf("Marshalable!\n")
		m, ok := val.Interface().(Marshaler)
		if ok {
			return m.DOMETA(ctx, e)
		}
	}

	switch k := val.Kind(); k {
	case reflect.Map:
    var map_err error
		//fmt.Printf("Map\n")

		var ret reflect.Value
		keyType := val.Type().Key()
		elemType := val.Type().Elem()
		maptype := reflect.MapOf(keyType, elemType)
		ret = reflect.MakeMap(maptype)

		m := sync.Mutex{}
		wg := sync.WaitGroup{}
		for _, k := range val.MapKeys() {
			wg.Add(1)
			func(k reflect.Value) {
				newEncOpts := encOpts{
					request:   e.request,
					present:   e.present,
					process:   e.process,
					fnArgAuth: e.fnArgAuth,
				}

				// if e.request has any data, pull out the matching mapval
				requestBody, ok := newEncOpts.request.(map[string]interface{})
				newEncOpts.present = ok
				if newEncOpts.present {
					newEncOpts.request, newEncOpts.present = requestBody[k.Interface().(string)]
				}
				mapVal := val.MapIndex(k).Interface()
				parsed, err := parseRecursive(ctx, reflect.ValueOf(mapVal), newEncOpts)
        if err != nil {
          map_err = err
        }

				if !parsed.IsValid() {
					// SetMapIndex will *delete* the indexed entry if you pass a nil!
					parsed = reflect.Zero(elemType)
				}
				m.Lock()
				ret.SetMapIndex(k, parsed)
				m.Unlock()
				wg.Done()
			}(k)
		}
		wg.Wait()
		return ret, map_err

	case reflect.Slice:
		//fmt.Printf("slice\n")

		var ret reflect.Value
    var map_err error
		elemType := val.Type().Elem()
		arraytype := reflect.SliceOf(elemType)
		ret = reflect.MakeSlice(arraytype, val.Len(), val.Cap())

		wg := sync.WaitGroup{}
		for i := 0; i < val.Len(); i++ {
			wg.Add(1)
			func(k int) {
				defer wg.Done()
				sliceVal := val.Index(k)
				if sliceVal.IsValid() {
					parsed, err := parseRecursive(ctx, reflect.ValueOf(sliceVal.Interface()), e)
          if err != nil {
            map_err = err
          }
					ret.Index(k).Set(parsed)
				}
			}(i)
		}
		wg.Wait()
		return ret, map_err

	default:
		//fmt.Printf("other\n")
	}

	return val, nil
}

type NewPropGetter interface {
	PropertyGet(context.Context, *RedfishAuthorizationProperty, *RedfishResourceProperty, map[string]interface{}) error
}

type NewPropPatcher interface {
	PropertyPatch(context.Context, *RedfishAuthorizationProperty, *RedfishResourceProperty, interface{}, map[string]interface{}) (interface{}, error)
}
type CompatPropPatcher interface {
	PropertyPatch(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{})
}

func GETfn(ctx context.Context, rrp *RedfishResourceProperty, opts encOpts) (interface{}, error) {
	meta_t, ok := rrp.Meta["GET"].(map[string]interface{})
	if !ok {
		return rrp.Value, errors.New("No GET")
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		return rrp.Value, errors.New("No plugin in GET")
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		return rrp.Value, errors.New("No plugin named(" + pluginName + ") for GET")
	}

	// comment out debugging in the fast path. Uncomment if you need to debug
	// ContextLogger(ctx, "property_process").Debug("getting property: GET", "value", fmt.Sprintf("%v", rrp.Value))
	if plugin, ok := plugin.(NewPropGetter); ok {
		// comment out debugging in the fast path. Uncomment if you need to debug
		//defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: GET - type assert success", "value", fmt.Sprintf("%v", rrp.Value))

		// plugin can use value to cache if it resets this
		rrp.Ephemeral = true
		err = plugin.PropertyGet(ctx, opts.fnArgAuth, rrp, meta_t)
	}
	ret := rrp.Value
	if rrp.Ephemeral {
		rrp.Value = nil
	}
	return ret, err
}

func PATCHfn(ctx context.Context, rrp *RedfishResourceProperty, opts encOpts) (interface{}, error) {
	ContextLogger(ctx, "property_process").Debug("PATCHfn", "opts", opts, "rrp", rrp)
	if !opts.present {
		ContextLogger(ctx, "property_process").Debug("NOT PRESENT")
		return GETfn(ctx, rrp, opts)
	}

	meta_t, ok := rrp.Meta["PATCH"].(map[string]interface{})
	if !ok {
		ContextLogger(ctx, "property_process").Debug("No PATCH meta", "meta", meta_t)
		return GETfn(ctx, rrp, opts)
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		ContextLogger(ctx, "property_process").Debug("No pluginname in patch meta", "meta", meta_t)
		return GETfn(ctx, rrp, opts)
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		ContextLogger(ctx, "property_process").Debug("No such pluginname", "pluginName", pluginName)
		return GETfn(ctx, rrp, opts)
	}

	//ContextLogger(ctx, "property_process").Debug("getting property: PATCH", "value", fmt.Sprintf("%v", rrp.Value), "plugin", plugin)
	if plugin, ok := plugin.(NewPropPatcher); ok {
		//defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		return plugin.PropertyPatch(ctx, opts.fnArgAuth, rrp, opts.request, meta_t)
	}
	if plugin, ok := plugin.(CompatPropPatcher); ok {
		//defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		tempRRP := &RedfishResourceProperty{Value: rrp.Value, Meta: rrp.Meta}
		plugin.PropertyPatch(ctx, nil, tempRRP, meta_t)
		return tempRRP.Value, nil
	}
	return rrp.Value, errors.New("foobar")
}
