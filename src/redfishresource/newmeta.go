package domain

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

type processFn func(context.Context, *RedfishResourceProperty, encOpts) (interface{}, error)

type encOpts struct {
	request interface{}
	present bool
	process processFn
}

func ProcessPATCH(ctx context.Context, prop *RedfishResourceProperty, request interface{}) (results interface{}, err error) {
	opts := encOpts{
		request: request,
		process: PATCHfn,
	}

	val, err := parseRecursive(ctx, reflect.ValueOf(prop), opts)
	if val.IsValid() {
		return val.Interface(), err
	}
	return nil, err
}

func ProcessGET(ctx context.Context, prop *RedfishResourceProperty) (results interface{}, err error) {
	opts := encOpts{
		request: nil,
		process: GETfn,
	}

	val, err := parseRecursive(ctx, reflect.ValueOf(*prop), opts)
	if val.IsValid() {
		return val.Interface(), err
	}
	return nil, err
}

type Marshaler interface {
	DOMETA(context.Context, encOpts) (reflect.Value, error)
}

func (rrp *RedfishResourceProperty) DOMETA(ctx context.Context, e encOpts) (results reflect.Value, err error) {
	//fmt.Printf("DOMETA\n")
	res, _ := e.process(ctx, rrp, e)
	return parseRecursive(ctx, reflect.ValueOf(res), e)
}

var marshalerType = reflect.TypeOf(new(Marshaler)).Elem()

func parseRecursive(ctx context.Context, val reflect.Value, e encOpts) (reflect.Value, error) {
	if !val.IsValid() {
		//fmt.Printf("NOT VALID, returning\n")
		return val, errors.New("not a valid type")
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
			go func(k reflect.Value) {
				newEncOpts := encOpts{
					request: e.request,
					present: e.present,
					process: e.process,
				}

				// if e.request has any data, pull out the matching mapval
				requestBody, ok := newEncOpts.request.(map[string]interface{})
				newEncOpts.present = ok
				if newEncOpts.present {
					newEncOpts.request, newEncOpts.present = requestBody[k.Interface().(string)]
				}

				mapVal := val.MapIndex(k).Interface()
				parsed, err := parseRecursive(ctx, reflect.ValueOf(mapVal), newEncOpts)
				_ = err // supress unused var error

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
		return ret, nil

	case reflect.Slice:
		//fmt.Printf("slice\n")

		var ret reflect.Value
		elemType := val.Type().Elem()
		arraytype := reflect.SliceOf(elemType)
		ret = reflect.MakeSlice(arraytype, val.Len(), val.Cap())

		wg := sync.WaitGroup{}
		for i := 0; i < val.Len(); i++ {
			wg.Add(1)
			go func(k int) {
				//TODO: for PATCH, no clue how we map an array of body elements to an array here! Punting for now

				sliceVal := val.Index(k)
				parsed, err := parseRecursive(ctx, reflect.ValueOf(sliceVal.Interface()), e)
				_ = err // supress unused var error
				ret.Index(k).Set(parsed)
				wg.Done()
			}(i)
		}
		wg.Wait()
		return ret, nil

	default:
		//fmt.Printf("other\n")
	}

	return val, nil
}

type NewPropGetter interface {
	PropertyGet(context.Context, *RedfishResourceProperty, map[string]interface{}) error
}

type NewPropPatcher interface {
	PropertyPatch(context.Context, *RedfishResourceProperty, interface{}, map[string]interface{}) (interface{}, error)
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

	ContextLogger(ctx, "property_process").Debug("getting property: GET", "value", fmt.Sprintf("%v", rrp.Value))
	if plugin, ok := plugin.(NewPropGetter); ok {
		defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: GET - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		err = plugin.PropertyGet(ctx, rrp, meta_t)
	}
	return rrp.Value, err
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

	ContextLogger(ctx, "property_process").Debug("getting property: PATCH", "value", fmt.Sprintf("%v", rrp.Value), "plugin", plugin)
	if plugin, ok := plugin.(NewPropPatcher); ok {
		defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		return plugin.PropertyPatch(ctx, rrp, opts.request, meta_t)
	}
	if plugin, ok := plugin.(CompatPropPatcher); ok {
		defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		tempRRP := &RedfishResourceProperty{Value: rrp.Value, Meta: rrp.Meta}
		plugin.PropertyPatch(ctx, nil, tempRRP, meta_t)
		return tempRRP.Value, nil
	}
	return rrp.Value, errors.New("foobar")
}
