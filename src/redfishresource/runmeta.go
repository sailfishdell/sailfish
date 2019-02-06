package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

func (rrp *RedfishResourceProperty) MarshalJSON() ([]byte, error) {
	rrp.RLock()
	defer rrp.RUnlock()
	return json.Marshal(rrp.Value)
}

func NewGet(ctx context.Context, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty) (err error) {
	opts := nuEncOpts{
		request: nil,
		process: nuGETfn,
	}

	return rrp.RunMetaFunctions(ctx, auth, opts)
}

type nuProcessFn func(context.Context, *RedfishResourceProperty, *RedfishAuthorizationProperty, nuEncOpts) error

type nuEncOpts struct {
	request interface{}
	present bool
	process nuProcessFn
}

func nuGETfn(ctx context.Context, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, opts nuEncOpts) error {
	meta_t, ok := rrp.Meta["GET"].(map[string]interface{})
	if !ok {
		return errors.New("No GET")
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		return errors.New("No plugin in GET")
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		return errors.New("No plugin named(" + pluginName + ") for GET")
	}

	if plugin, ok := plugin.(NewPropGetter); ok {
		rrp.Ephemeral = true
		err = plugin.PropertyGet(ctx, auth, rrp, meta_t)
	}
	return err
}

type stopProcessing interface {
	ShouldStop() bool
}

// this should always be string/int/float, or map/slice. There should never be pointers or other odd data structures in a rrp.
func Flatten(thing interface{}) interface{} {
	// if it's an rrp, return the value
	if vp, ok := thing.(*RedfishResourceProperty); ok {
		v := vp.Value
		if vp.Ephemeral {
			vp.Value = nil
		}
		return Flatten(v)
	}

	// recurse through maps or slices and recursively call helper on them
	val := reflect.ValueOf(thing)
	switch k := val.Kind(); k {
	case reflect.Ptr:
		fmt.Printf("PTR!\n")

	case reflect.Map:
		// everything inside of a redfishresourceproperty should fit into a map[string]interface{}
		ret := map[string]interface{}{}
		for _, k := range val.MapKeys() {
			s, ok := k.Interface().(string)
			v := val.MapIndex(k)
			if ok && v.IsValid() {
				ret[s] = Flatten(v.Interface())
			}
		}
		return ret

	case reflect.Slice:
		// squash every type of array into an []interface{}
		ret := make([]interface{}, val.Len())
		for i := 0; i < val.Len(); i++ {
			sliceVal := val.Index(i)
			if sliceVal.IsValid() {
				ret[i] = Flatten(sliceVal.Interface())
			}
		}
		return ret

	default:
		return thing
	}

	return nil
}

func (rrp *RedfishResourceProperty) RunMetaFunctions(ctx context.Context, auth *RedfishAuthorizationProperty, e nuEncOpts) (err error) {
	rrp.Lock()
	defer rrp.Unlock()

	err = e.process(ctx, rrp, auth, e)
	if a, ok := err.(stopProcessing); ok && a.ShouldStop() {
		return
	}

	helperError := helper(ctx, auth, e, rrp.Value)

	if err != nil {
		return err
	} else {
		return helperError
	}
}

type AddAnnotation interface {
	GetAnnotations() map[string]interface{}
}

func helper(ctx context.Context, auth *RedfishAuthorizationProperty, e nuEncOpts, v interface{}) error {
	// handle special case of RRP inside RRP.Value of parent
	if vp, ok := v.(*RedfishResourceProperty); ok {
		return vp.RunMetaFunctions(ctx, auth, e)
	}

	// recurse through maps or slices and recursively call helper on them
	val := reflect.ValueOf(v)
	switch k := val.Kind(); k {
	case reflect.Map:
		keyType := val.Type().Key()
		elemType := val.Type().Elem()
		for _, k := range val.MapKeys() {
			newEncOpts := nuEncOpts{
				request: e.request,
				present: e.present,
				process: e.process,
			}

			requestBody, ok := newEncOpts.request.(map[string]interface{})
			newEncOpts.present = ok
			if newEncOpts.present {
				newEncOpts.request, newEncOpts.present = requestBody[k.Interface().(string)]
			}

			mapVal := val.MapIndex(k)
			if mapVal.IsValid() {
				err := helper(ctx, auth, newEncOpts, mapVal.Interface())
				if err == nil {
					continue
				}
				e, ok := err.(AddAnnotation)
				if !ok {
					// TODO: collate all the errors and pass up somehow?
					continue
				}

				// if there are any annotations returned, map those
				for k, v := range e.GetAnnotations() {
					if compatible(reflect.TypeOf(k), keyType) && compatible(reflect.TypeOf(v), elemType) {
						val.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
					}
				}
			}
		}

		return nil

	case reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			sliceVal := val.Index(i)
			if sliceVal.IsValid() {
				helper(ctx, auth, e, sliceVal.Interface())
				// TODO: do annotations make sense here?
			}
		}
		return nil
	}

	return nil
}

func compatible(actual, expected reflect.Type) bool {
	if actual == nil {
		k := expected.Kind()
		return k == reflect.Chan ||
			k == reflect.Func ||
			k == reflect.Interface ||
			k == reflect.Map ||
			k == reflect.Ptr ||
			k == reflect.Slice
	}
	return actual.AssignableTo(expected)
}
