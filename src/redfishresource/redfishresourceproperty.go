package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/superchalupa/sailfish/src/dell-resources/dellauth"
)

type RedfishAuthorizationProperty struct {
	UserName   string
	Privileges []string
	Licenses   []string
}

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

func (auth *RedfishAuthorizationProperty) VerifyPrivileges(Privileges []string) bool {

	rh := &RedfishHandler{
		UserName:   auth.UserName,
		Privileges: auth.Privileges,
	}

	authAction := rh.isAuthorized(Privileges)

	if authAction != "authorized" {
		fmt.Println("required auth ", Privileges)
		fmt.Println(auth.UserName, " current priv ", auth.Privileges)
		fmt.Println("allowed ", authAction)
	}

	return authAction == "authorized"
}

func (auth *RedfishAuthorizationProperty) VerifyPrivilegeBits(requiredPrivs int) bool {
	// TODO: remove once the privlige information is being sent as an array of strings.
	Privileges := dellauth.PrivilegeBitsToStrings(requiredPrivs)
	return auth.VerifyPrivileges(Privileges)
}
