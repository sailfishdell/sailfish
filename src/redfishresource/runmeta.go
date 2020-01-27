package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"

	"reflect"

	"github.com/superchalupa/sailfish/src/log"
)

func (rrp *RedfishResourceProperty) MarshalJSON() ([]byte, error) {
	rrp.RLock()
	defer rrp.RUnlock()

	return json.Marshal(rrp.Value)
}

func NewGet(ctx context.Context, agg *RedfishResourceAggregate, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty) error {

	opts := NuEncOpts{
		Request: nil,
		process: nuGETfn,
		root:    true,
		sel:     auth.sel,
		path:    "",
	}

	return rrp.runMetaFunctions(ctx, agg, auth, opts)
}

func NewPatch(ctx context.Context, response map[string]interface{}, agg *RedfishResourceAggregate, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, body interface{}) error {
	// Paste in redfish spec stuff here
	// 200 if anything succeeds, 400 if everything fails
	opts := NuEncOpts{
		Request:      body,
		Parse:        body,
		process:      nuPATCHfn,
		HttpResponse: response,
		root:         true,
		path:         "",
	}

	return rrp.runMetaFunctions(ctx, agg, auth, opts)
}

type nuProcessFn func(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, *RedfishAuthorizationProperty, NuEncOpts) error

type NuEncOpts struct {
	root         bool
	Parse        interface{}
	Request      interface{}
	present      bool
	HttpResponse map[string]interface{}
	process      nuProcessFn
	path         string
	sel          []string
}
type PropGetter interface {
	PropertyGet(context.Context, *RedfishResourceAggregate, *RedfishAuthorizationProperty, *RedfishResourceProperty, map[string]interface{}) error
}


func nuGETfn(ctx context.Context, agg *RedfishResourceAggregate, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, opts NuEncOpts) error {
	meta_t, ok := rrp.Meta["GET"].(map[string]interface{})
	if !ok {
		return nil // it's not really an "error" we need upper layers to care about
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		// default to a plugin named after this URI to simplify creating aggregates
		pluginName = agg.ResourceURI
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		return errors.New("No plugin named(" + pluginName + ") for GET")
	}

	if plugin, ok := plugin.(PropGetter); ok {
		rrp.Ephemeral = true
		err = plugin.PropertyGet(ctx, agg, auth, rrp, meta_t)
	}
	return err
}

type PropPatcher interface {
	PropertyPatch(context.Context, *RedfishResourceAggregate, *RedfishAuthorizationProperty, *RedfishResourceProperty, *NuEncOpts, map[string]interface{}) error
}

func nuPATCHfn(ctx context.Context, agg *RedfishResourceAggregate, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, opts NuEncOpts) error {
	if opts.Request != nil {
		if req_map, ok := opts.Request.(map[string]interface{}); ok {
			if val, ok := req_map["ERROR"]; ok {
				valStr, ok := val.(string)
				if !ok {
					fmt.Println("something is wrong")
					return nil
				}
				AddEEMIMessage(opts.HttpResponse, agg, valStr, nil)
				return nil
			}
		}
	}
	if !opts.present {
		return nuGETfn(ctx, agg, rrp, auth, opts)
	}

	meta_t, ok := rrp.Meta["PATCH"].(map[string]interface{})
	if !ok {
		log.ContextLogger(ctx, "property_process").Debug("No PATCH meta", "meta", meta_t)
		return nuGETfn(ctx, agg, rrp, auth, opts)
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		log.ContextLogger(ctx, "property_process").Debug("No pluginname in patch meta", "meta", meta_t)
		return nuGETfn(ctx, agg, rrp, auth, opts)
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		log.ContextLogger(ctx, "property_process").Debug("No such pluginname", "pluginName", pluginName)
		return nuGETfn(ctx, agg, rrp, auth, opts)
	}

	//log.ContextLogger(ctx, "property_process").Debug("getting property: PATCH", "value", fmt.Sprintf("%v", rrp.Value), "plugin", plugin)
	if plugin, ok := plugin.(PropPatcher); ok {
		//defer log.ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		plugin.PropertyPatch(ctx, agg, auth, rrp, &opts, meta_t)
	} else {
		panic("coding error: the plugin " + pluginName + " does not implement the Property Patching API")
	}
	return nil
}

type stopProcessing interface {
	ShouldStop() bool
}

// this should always be string/int/float, or map/slice. There should never be pointers or other odd data structures in a rrp.
// aggregate Properties are passed in as thing
func Flatten(thing interface{}, parentlocked bool) interface{} {
	// if it's an rrp, return the value
	if vp, ok := thing.(*RedfishResourceProperty); ok {
		if vp.Ephemeral {
			vp.RLock()
			defer vp.RUnlock()

		} else {
			vp.Lock()
			defer vp.Unlock()

		}

		ret := Flatten(vp.Value, true) // the only instance where we deref value
		if vp.Ephemeral {
			vp.Value = nil
		}
		return ret
	}

	// recurse through maps or slices and recursively call helper on them
	val := reflect.ValueOf(thing)
	switch k := val.Kind(); k {
	case reflect.Ptr:
		fmt.Printf("ERROR: Detected a pointer in the redfish resource property tree. This is not allowed.\n")
		return nil

	case reflect.Map:
		// everything inside of a redfishresourceproperty should fit into a map[string]interface{}
		if !parentlocked {
			fmt.Printf("ERROR: detected a nested map inside the redfish resource property tree. This is not allowed, wrap the child in an &RedfishResourceProperty{Value: ...}\n")
			fmt.Printf("ERROR: The offending keys were: %s\n", val.String())
		}

		ret := map[string]interface{}{}
		for _, k := range val.MapKeys() {
			s, ok := k.Interface().(string)
			v := val.MapIndex(k)
			if ok && v.IsValid() {
				ret[s] = Flatten(v.Interface(), false)
			}
		}
		return ret

	case reflect.Slice:
		if !parentlocked {
			fmt.Printf("ERROR: detected a nested array inside the redfish resource property tree. This is not allowed, wrap the child in an &RedfishResourceProperty{Value: ...}\n")
			fmt.Printf("ERROR: The offending keys were: %s\n", val.String())
		}
		// squash every type of array into an []interface{}
		ret := make([]interface{}, val.Len())
		for i := 0; i < val.Len(); i++ {
			sliceVal := val.Index(i)
			if sliceVal.IsValid() {
				ret[i] = Flatten(sliceVal.Interface(), false)
			}
		}
		return ret

	default:
		return thing
	}
}

func (rrp *RedfishResourceProperty) runMetaFunctions(ctx context.Context, agg *RedfishResourceAggregate, auth *RedfishAuthorizationProperty, e NuEncOpts) (err error) {
	rrp.Lock()
	defer rrp.Unlock()

	err = e.process(ctx, agg, rrp, auth, e)
	if a, ok := err.(stopProcessing); ok && a.ShouldStop() {
		return
	}

	helper(ctx, agg, auth, e, rrp.Value)
	// TODO: need to collect messages here

	if err != nil {
		return err
	}
	return nil
}

func helper(ctx context.Context, agg *RedfishResourceAggregate, auth *RedfishAuthorizationProperty, encopts NuEncOpts, v interface{}) error {
	var ok bool
	// handle special case of RRP inside RRP.Value of parent
	if vp, ok := v.(*RedfishResourceProperty); ok {
		return vp.runMetaFunctions(ctx, agg, auth, encopts)
	}

	// recurse through maps or slices and recursively call helper on them
	val := reflect.ValueOf(v)
	switch k := val.Kind(); k {
	case reflect.Map:

		for _, k := range val.MapKeys() {
			newEncOpts := NuEncOpts{
				Request:      encopts.Request,
				Parse:        encopts.Parse,
				present:      encopts.present,
				HttpResponse: encopts.HttpResponse,
				process:      encopts.process,
				root:         false,
				path:         path.Join(encopts.path, k.String()),
			}

			// Header information needs to always have the meta expanded
			if isHeader(newEncOpts.path) {
			} else if auth.doSel && encopts.sel != nil {
				ok, newEncOpts.sel = selectCheck(newEncOpts.path, encopts.sel, auth.selT)
				if !ok {
					continue
				}
			}

			parseBody, ok := newEncOpts.Parse.(map[string]interface{})
			newEncOpts.present = ok
			if newEncOpts.present {
				newEncOpts.Parse, newEncOpts.present = parseBody[k.Interface().(string)]
			}
			if newEncOpts.Parse == nil && k.Interface().(string) == "Attributes" {
				newEncOpts.Parse = map[string]interface{}{"ERROR": "BADREQUEST"}
			}
			mapVal := val.MapIndex(k)
			if mapVal.IsValid() {
				err := helper(ctx, agg, auth, newEncOpts, mapVal.Interface())
				if err == nil {
					continue
				}
			}
		}

	case reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			sliceVal := val.Index(i)
			if sliceVal.IsValid() {
				err := helper(ctx, agg, auth, encopts, sliceVal.Interface())
				if err == nil {
					continue
				}
			}
		}
	}

	return nil
}
