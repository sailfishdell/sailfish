package domain

import (
	"context"
	"errors"
	"reflect"
	"sync"
	//"fmt"
)

type processFn func(context.Context, RedfishResourceProperty, encOpts) (interface{}, error)

type encOpts struct {
	request map[string]interface{}
	process processFn
}

func ProcessPATCH(ctx context.Context, prop RedfishResourceProperty, request map[string]interface{}) (results interface{}, err error) {
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

func ProcessGET(ctx context.Context, prop RedfishResourceProperty) (results interface{}, err error) {
	opts := encOpts{
		request: nil,
		process: GETfn,
	}

	val, err := parseRecursive(ctx, reflect.ValueOf(prop), opts)
	if val.IsValid() {
		return val.Interface(), err
	}
	return nil, err
}

type Marshaler interface {
	DOMETA(context.Context, encOpts) (reflect.Value, error)
}

func (rrp RedfishResourceProperty) DOMETA(ctx context.Context, e encOpts) (results reflect.Value, err error) {
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
				mapVal := val.MapIndex(k).Interface()
				parsed, err := parseRecursive(ctx, reflect.ValueOf(mapVal), e)
				_ = err // supress unused var error
				//if err != nil {
				//fmt.Printf("map parseRecursive returned key: %s error: %s for val: %#v  parsed: %#v\n", k, err.Error(), mapVal, parsed)
				//}

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
				sliceVal := val.Index(k)
				parsed, err := parseRecursive(ctx, reflect.ValueOf(sliceVal.Interface()), e)
				_ = err // supress unused var error
				//if err != nil {
				//fmt.Printf("slice parseRecursive returned error: %s for val: %#v\n", err.Error(), sliceVal)
				//}
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
	PropertyGet(context.Context, RedfishResourceProperty, map[string]interface{}) (interface{}, error)
}
type CompatPropGetter interface {
	PropertyGet(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{})
}

type NewPropPatcher interface {
	PropertyPatch(context.Context, RedfishResourceProperty, map[string]interface{}, map[string]interface{}) (interface{}, error)
}
type CompatPropPatcher interface {
	PropertyPatch(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{})
}

func GETfn(ctx context.Context, rrp RedfishResourceProperty, opts encOpts) (interface{}, error) {
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

	// ContextLogger(ctx, "property_process").Debug("getting property: GET", "value", fmt.Sprintf("%v", rrp.Value))
	if plugin, ok := plugin.(NewPropGetter); ok {
		// defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: GET - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		return plugin.PropertyGet(ctx, rrp, meta_t)
	}
	if plugin, ok := plugin.(CompatPropGetter); ok {
		// defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: GET - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		tempRRP := &RedfishResourceProperty{Value: rrp.Value, Meta: rrp.Meta}
		plugin.PropertyGet(ctx, nil, tempRRP, meta_t)
		return tempRRP.Value, nil
	}
	return rrp.Value, errors.New("foobar")
}

func PATCHfn(ctx context.Context, rrp RedfishResourceProperty, opts encOpts) (interface{}, error) {
	meta_t, ok := rrp.Meta["PATCH"].(map[string]interface{})
	if !ok {
		return rrp.Value, errors.New("No PATCH")
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		return rrp.Value, errors.New("No plugin in PATCH")
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		return rrp.Value, errors.New("No plugin named(" + pluginName + ") for PATCH")
	}

	// ContextLogger(ctx, "property_process").Debug("getting property: PATCH", "value", fmt.Sprintf("%v", rrp.Value))
	if plugin, ok := plugin.(NewPropPatcher); ok {
		// defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		return plugin.PropertyPatch(ctx, rrp, opts.request, meta_t)
	}
	if plugin, ok := plugin.(CompatPropPatcher); ok {
		// defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		tempRRP := &RedfishResourceProperty{Value: rrp.Value, Meta: rrp.Meta}
		plugin.PropertyPatch(ctx, nil, tempRRP, meta_t)
		return tempRRP.Value, nil
	}
	return rrp.Value, errors.New("foobar")
}
