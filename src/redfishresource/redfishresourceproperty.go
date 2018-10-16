package domain

import (
	"encoding/json"
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

func (rrp *RedfishResourceProperty) MarshalJSON() ([]byte, error) {
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
