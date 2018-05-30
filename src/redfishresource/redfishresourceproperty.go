package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type RedfishResourceProperty struct {
	sync.Mutex
	Value interface{}
	Meta  map[string]interface{}
}

func NewProperty() *RedfishResourceProperty {
	return &RedfishResourceProperty{}
}

func (rrp RedfishResourceProperty) MarshalJSON() ([]byte, error) {
	rrp.Lock()
	defer rrp.Unlock()
	return json.Marshal(rrp.Value)
}

func (rrp *RedfishResourceProperty) Parse(thing interface{}) (ret *RedfishResourceProperty) {
	rrp.Lock()
	defer rrp.Unlock()
	ret = rrp
	switch thing.(type) {
	case []interface{}:
		if _, ok := rrp.Value.([]interface{}); !ok || rrp.Value == nil {
			rrp.Value = []interface{}{}
		}
		rrp.Value = append(rrp.Value.([]interface{}), parse_array(thing.([]interface{}))...)
	case map[string]interface{}:
		v, ok := rrp.Value.(map[string]interface{})
		if !ok || v == nil {
			rrp.Value = map[string]interface{}{}
		}
		parse_map(rrp.Value.(map[string]interface{}), thing.(map[string]interface{}))
	default:
		rrp.Value = thing
	}
	return
}

func parse_array(props []interface{}) (ret []interface{}) {
	for _, v := range props {
		prop := &RedfishResourceProperty{}
		prop.Parse(v)
		ret = append(ret, prop)
	}
	return
}

func parse_map(start map[string]interface{}, props map[string]interface{}) {
	for k, v := range props {
		if strings.HasSuffix(k, "@meta") {
			name := k[:len(k)-5]
			prop, ok := start[name].(*RedfishResourceProperty)
			if !ok {
				prop = &RedfishResourceProperty{}
			}
			prop.Meta = v.(map[string]interface{})
			start[name] = prop
		} else {
			prop, ok := start[k].(*RedfishResourceProperty)
			if !ok {
				prop = &RedfishResourceProperty{}
			}
			prop.Parse(v)
			start[k] = prop
		}
	}
	return
}

type PropertyGetter interface {
	PropertyGet(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{})
}
type PropertyPatcher interface {
	PropertyPatch(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{}, interface{}, bool)
}

func (rrp *RedfishResourceProperty) Process(ctx context.Context, agg *RedfishResourceAggregate, property, method string, req interface{}, present bool) {
	rrp.Lock()
	defer rrp.Unlock()

	// The purpose of this function is to recursively process the resource property, calling any plugins that are specified in the meta data.
	// step 1: run the plugin to update rrp.Value based on the plugin.
	// Step 2: see if the rrp.Value is a recursable map or array and recurse down it

	// equivalent to do{}while(1) to run once
	// if any of the intermediate steps fails, bail out on this part and continue by doing the next thing
	for ok := true; ok; ok = false {
		meta_t, ok := rrp.Meta[method].(map[string]interface{})
		if !ok {
			break
		}

		pluginName, ok := meta_t["plugin"].(string)
		if !ok {
			break
		}

		plugin, err := InstantiatePlugin(PluginType(pluginName))
		if err != nil {
			ContextLogger(ctx, "property_process").Warn("Orphan property, could not load plugin", "property", property, "plugin", pluginName, "err", err)
			break
		}

		ContextLogger(ctx, "property_process").Debug("getting property", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
		switch method {
		case "GET":
			ContextLogger(ctx, "property_process").Debug("getting property: GET", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
			if plugin, ok := plugin.(PropertyGetter); ok {
				ContextLogger(ctx, "property_process").Debug("getting property: GET - type assert success", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
				plugin.PropertyGet(ctx, agg, rrp, meta_t)
				ContextLogger(ctx, "property_process").Debug("AFTER getting property: GET - type assert success", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
			}
		case "PATCH":
			ContextLogger(ctx, "property_process").Debug("getting property: PATCH", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
			if plugin, ok := plugin.(PropertyPatcher); ok {
				ContextLogger(ctx, "property_process").Debug("getting property: PATCH - type assert success", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
				plugin.PropertyPatch(ctx, agg, rrp, meta_t, req, present)
				ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
			}
		}

		ContextLogger(ctx, "property_process").Debug("GOT Property", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp), "rrp.Value.TYPE", fmt.Sprintf("%T", rrp.Value))

	}

	switch t := rrp.Value.(type) {
	case map[string]interface{}:
		ContextLogger(ctx, "property_process").Debug("Handle MAP", "rrp", rrp, "TYPE", fmt.Sprintf("%T", t))
		var wg sync.WaitGroup
		for property, v := range t {
			if vrr, ok := v.(*RedfishResourceProperty); ok {
				wg.Add(1)
				go func(property string, v *RedfishResourceProperty) {
					defer wg.Done()
					reqmap, ok := req.(map[string]interface{})
					var reqitem interface{}
					if ok {
						reqitem, ok = reqmap[property]
					}
					v.Process(ctx, agg, property, method, reqitem, ok)
				}(property, vrr)
			}
		}
		wg.Wait()
		ContextLogger(ctx, "property_process").Debug("DONE Handle MAP", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))

	case []interface{}:
		ContextLogger(ctx, "property_process").Debug("Handle ARRAY", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))
		// spawn off parallel goroutines to process each member of the array
		var wg sync.WaitGroup
		for index, v := range rrp.Value.([]interface{}) {
			if v, ok := v.(*RedfishResourceProperty); ok {
				wg.Add(1)
				go func(index int, v *RedfishResourceProperty) {
					defer wg.Done()
					var reqitem interface{} = nil
					var ok bool = false
					reqarr, ok := req.([]interface{})
					if ok {
						if index < len(reqarr) {
							reqitem = reqarr[index]
							ok = true
						}
					}
					v.Process(ctx, agg, property, method, reqitem, ok)
				}(index, v)
			}
		}
		wg.Wait()
		ContextLogger(ctx, "property_process").Debug("DONE Handle ARRAY", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))

	case *RedfishResourceProperty:
		ContextLogger(ctx, "property_process").Debug("Handle single", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))
		if v, ok := rrp.Value.(*RedfishResourceProperty); ok {
			v.Process(ctx, agg, property, method, req, true)
		}
		ContextLogger(ctx, "property_process").Debug("DONE Handle single", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))

	default:
		ContextLogger(ctx, "property_process").Debug("CANT HANDLE", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))

	}

	ContextLogger(ctx, "property_process").Debug("ALL DONE: GOT Property", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value), "Value", fmt.Sprintf("%+v", rrp.Value))

	return
}
