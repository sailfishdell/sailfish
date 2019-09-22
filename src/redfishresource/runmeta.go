package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"

	"reflect"
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

	return rrp.RunMetaFunctions(ctx, agg, auth, opts)
}

func NewPatch(ctx context.Context, agg *RedfishResourceAggregate, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, body interface{}) error {
	// Paste in redfish spec stuff here
	// 200 if anything succeeds, 400 if everything fails
	opts := NuEncOpts{
		Request: body,
		Parse:   body,
		process: nuPATCHfn,
		root:    true,
		path:    "",
	}

	return rrp.RunMetaFunctions(ctx, agg, auth, opts)
}

type nuProcessFn func(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, *RedfishAuthorizationProperty, NuEncOpts) error

type NuEncOpts struct {
	root    bool
	Parse   interface{}
	Request interface{}
	present bool
	process nuProcessFn
	path    string
	sel     []string
}

func nuGETfn(ctx context.Context, agg *RedfishResourceAggregate, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, opts NuEncOpts) error {
	meta_t, ok := rrp.Meta["GET"].(map[string]interface{})
	if !ok {
		return nil // it's not really an "error" we need upper layers to care about
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		return errors.New("No plugin in GET")
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

type PropGetter interface {
	PropertyGet(context.Context, *RedfishResourceAggregate, *RedfishAuthorizationProperty, *RedfishResourceProperty, map[string]interface{}) error
}

type PropPatcher interface {
	PropertyPatch(context.Context, *RedfishResourceAggregate, *RedfishAuthorizationProperty, *RedfishResourceProperty, *NuEncOpts, map[string]interface{}) error
}

func nuPATCHfn(ctx context.Context, agg *RedfishResourceAggregate, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, opts NuEncOpts) error {

	bad_json := ExtendedInfo{
		Message:             "The request body submitted was malformed JSON and could not be parsed by the receiving service.",
		MessageArgs:         []string{}, //FIX ME
		MessageArgsCt:       0,          //FIX ME
		MessageId:           "Base.1.0.MalformedJSON",
		RelatedProperties:   []string{}, //FIX ME
		RelatedPropertiesCt: 0,          //FIX ME
		Resolution:          "Ensure that the request body is valid JSON and resubmit the request.",
		Severity:            "Critical",
	}

	bad_request := ExtendedInfo{
		Message:             "The service detected a malformed request body that it was unable to interpret.",
		MessageArgs:         []string{},
		MessageArgsCt:       0,
		MessageId:           "Base.1.0.UnrecognizedRequestBody",
		RelatedProperties:   []string{"Attributes"}, //FIX ME
		RelatedPropertiesCt: 1,                      //FIX ME
		Resolution:          "Correct the request body and resubmit the request if it failed.",
		Severity:            "Warning",
	}

	if opts.Request != nil {
		if req_map, ok := opts.Request.(map[string]interface{}); ok {
			if val, ok := req_map["ERROR"]; ok {
				var failed []interface{}
				if val == "BADJSON" {
					failed = append(failed, bad_json)
				}
				if val == "BADREQUEST" {
					failed = append(failed, bad_request)
				}
				return &CombinedPropObjInfoError{
					ObjectExtendedErrorMessages: *NewObjectExtendedErrorMessages(failed),
					NumSuccess:                  0,
				}
			}
		}
	}
	if !opts.present {
		return nuGETfn(ctx, agg, rrp, auth, opts)
	}

	meta_t, ok := rrp.Meta["PATCH"].(map[string]interface{})
	if !ok {
		ContextLogger(ctx, "property_process").Debug("No PATCH meta", "meta", meta_t)
		return nuGETfn(ctx, agg, rrp, auth, opts)
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		ContextLogger(ctx, "property_process").Debug("No pluginname in patch meta", "meta", meta_t)
		return nuGETfn(ctx, agg, rrp, auth, opts)
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		ContextLogger(ctx, "property_process").Debug("No such pluginname", "pluginName", pluginName)
		return nuGETfn(ctx, agg, rrp, auth, opts)
	}

	//ContextLogger(ctx, "property_process").Debug("getting property: PATCH", "value", fmt.Sprintf("%v", rrp.Value), "plugin", plugin)
	if plugin, ok := plugin.(PropPatcher); ok {
		//defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		return plugin.PropertyPatch(ctx, agg, auth, rrp, &opts, meta_t)
	} else {
		panic("coding error: the plugin " + pluginName + " does not implement the Property Patching API")
	}
}

type stopProcessing interface {
	ShouldStop() bool
}

// this should always be string/int/float, or map/slice. There should never be pointers or other odd data structures in a rrp.
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

func (rrp *RedfishResourceProperty) RunMetaFunctions(ctx context.Context, agg *RedfishResourceAggregate, auth *RedfishAuthorizationProperty, e NuEncOpts) (err error) {
	rrp.Lock()
	defer rrp.Unlock()

	err = e.process(ctx, agg, rrp, auth, e)
	if a, ok := err.(stopProcessing); ok && a.ShouldStop() {
		return
	}

	helperError := helper(ctx, agg, auth, e, rrp.Value)
	// TODO: need to collect messages here

	if err != nil {
		return err
	} else {
		return helperError
	}
}

type propertyExtMessages interface {
	GetPropertyExtendedMessages() []interface{}
}

type objectExtMessages interface {
	GetObjectExtendedMessages() []interface{}
}

type objectErrMessages interface {
	GetObjectErrorMessages() []interface{}
}

type numSuccess interface {
	GetNumSuccess() int
}

func helper(ctx context.Context, agg *RedfishResourceAggregate, auth *RedfishAuthorizationProperty, encopts NuEncOpts, v interface{}) error {
	var ok bool
	// handle special case of RRP inside RRP.Value of parent
	if vp, ok := v.(*RedfishResourceProperty); ok {
		return vp.RunMetaFunctions(ctx, agg, auth, encopts)
	}

	objectErrorMessages := []interface{}{}
	objectExtendedMessages := []interface{}{}
	anySuccess := 0

	// recurse through maps or slices and recursively call helper on them
	val := reflect.ValueOf(v)
	switch k := val.Kind(); k {
	case reflect.Map:

		elemType := val.Type().Elem()
		if encopts.root {
			annotatedKey := "@Message.ExtendedInfo"
			val.SetMapIndex(reflect.ValueOf(annotatedKey), reflect.Value{})
			val.SetMapIndex(reflect.ValueOf("error"), reflect.Value{})

		}

		for _, k := range val.MapKeys() {
			newEncOpts := NuEncOpts{
				Request: encopts.Request,
				Parse:   encopts.Parse,
				present: encopts.present,
				process: encopts.process,
				root:    false,
				path:    path.Join(encopts.path, k.String()),
			}

			// first scrub any old extended messages
			if strK, ok := k.Interface().(string); ok {
				annotatedKey := strK + "@Message.ExtendedInfo"
				val.SetMapIndex(reflect.ValueOf(annotatedKey), reflect.Value{})
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

				if e, ok := err.(numSuccess); ok {
					i := e.GetNumSuccess()
					if i > 0 {
						anySuccess = anySuccess + 1
					}
				}
				// annotate at this level
				propertyExtendedMessages := []interface{}{}
				if e, ok := err.(propertyExtMessages); ok {
					propertyExtendedMessages = append(propertyExtendedMessages, e.GetPropertyExtendedMessages()...)
				}
				// things to kick up a level
				if e, ok := err.(objectExtMessages); ok {
					objectExtendedMessages = append(objectExtendedMessages, e.GetObjectExtendedMessages()...)
				}
				if e, ok := err.(objectErrMessages); ok {
					objectErrorMessages = append(objectErrorMessages, e.GetObjectErrorMessages()...)
				}

				// TODO: add generic annotation support

				if len(propertyExtendedMessages) > 0 {
					if strK, ok := k.Interface().(string); ok {
						annotatedKey := strK + "@Message.ExtendedInfo"
						if compatible(reflect.TypeOf(propertyExtendedMessages), elemType) {
							val.SetMapIndex(reflect.ValueOf(annotatedKey), reflect.ValueOf(propertyExtendedMessages))
						}
					}
				}
			}
		}

		if encopts.root && len(objectExtendedMessages) > 0 {
			annotatedKey := "@Message.ExtendedInfo"
			if compatible(reflect.TypeOf(objectExtendedMessages), val.Type().Elem()) {
				val.SetMapIndex(reflect.ValueOf(annotatedKey), reflect.ValueOf(objectExtendedMessages))
			}
		}

		if encopts.root && len(objectErrorMessages) > 0 {
			if agg != nil {
				agg.StatusCode = 400
				if anySuccess > 0 {
					agg.StatusCode = 200
				}
			}
			annotatedKey := "error"
			value := map[string]interface{}{
				"code":                  "Base.1.0.GeneralError",
				"message":               "A general error has occurred. See ExtendedInfo for more information.",
				"@Message.ExtendedInfo": objectErrorMessages,
			}
			if compatible(reflect.TypeOf(value), val.Type().Elem()) {
				val.SetMapIndex(reflect.ValueOf(annotatedKey), reflect.ValueOf(value))
			}
		} else {
			if agg != nil {
				agg.StatusCode = 200
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

				if e, ok := err.(numSuccess); ok {
					i := e.GetNumSuccess()
					if i > 0 {
						anySuccess = anySuccess + 1
					}
				}
				// things to kick up a level
				if e, ok := err.(objectExtMessages); ok {
					objectExtendedMessages = append(objectExtendedMessages, e.GetObjectExtendedMessages()...)
				}
				if e, ok := err.(objectErrMessages); ok {
					objectErrorMessages = append(objectErrorMessages, e.GetObjectErrorMessages()...)
				}

				// TODO: do annotations make sense here?
			}
		}
	}

	return &CombinedPropObjInfoError{
		ObjectExtendedInfoMessages:  *NewObjectExtendedInfoMessages(objectExtendedMessages),
		ObjectExtendedErrorMessages: *NewObjectExtendedErrorMessages(objectErrorMessages),
		NumSuccess:                  anySuccess,
	}
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
