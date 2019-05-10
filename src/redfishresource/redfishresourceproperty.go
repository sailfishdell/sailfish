package domain

import (
	"net/url"
	"reflect"
	"strings"
	"sync"

	"github.com/superchalupa/sailfish/src/dell-resources/dellauth"
)

type RedfishAuthorizationProperty struct {
	UserName   string
	Privileges []string
	Licenses   []string
	Query      url.Values

	// pass the supported query options to the backend
	// re-arranged to hopefully be more memory efficient
	skip       int
	top        int
	filter     string
	sel        []string
	doFilter   bool
	filterDone bool
	doTop      bool
	topDone    bool
	doSkip     bool
	skipDone   bool
	doSel      bool
	selDone    bool
}

type RedfishResourceProperty struct {
	sync.RWMutex
	Value     interface{}
	Meta      map[string]interface{}
	Ephemeral bool
}

func (rrp *RedfishResourceProperty) Parse(thing interface{}) {
	rrp.Lock()
	defer rrp.Unlock()

	rrp.ParseUnlocked(thing)
}

func (rrp *RedfishResourceProperty) ParseUnlocked(thing interface{}) {
	val := reflect.ValueOf(thing)
	switch k := val.Kind(); k {
	case reflect.Map:
		v, ok := rrp.Value.(map[string]interface{})
		if !ok || v == nil {
			rrp.Value = map[string]interface{}{}
			v = rrp.Value.(map[string]interface{})
		}

		for _, k := range val.MapKeys() {
			rv := val.MapIndex(k)
			if !rv.IsValid() {
				continue
			}
			if strings.HasSuffix(k.String(), "@meta") {
				name := k.String()[:len(k.String())-5]
				newEntry := &RedfishResourceProperty{}
				if newEntry.Meta, ok = rv.Interface().(map[string]interface{}); ok {
					v[name] = newEntry
				}
			} else {
				newEntry := &RedfishResourceProperty{}
				newEntry.Parse(rv.Interface())
				v[k.String()] = newEntry
			}

		}

	case reflect.Slice:
		if _, ok := rrp.Value.([]interface{}); !ok || rrp.Value == nil {
			rrp.Value = make([]interface{}, 0, val.Len())
		}

		for i := 0; i < val.Len(); i++ {
			sliceVal := val.Index(i)
			if sliceVal.IsValid() {
				newEntry := &RedfishResourceProperty{}
				newEntry.Parse(sliceVal.Interface())
				rrp.Value = append(rrp.Value.([]interface{}), newEntry) //preallocated
			}
		}

	default:
		rrp.Value = thing
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
		//fmt.Println("required auth ", Privileges)
		//fmt.Println(auth.UserName, " current priv ", auth.Privileges)
		//fmt.Println("allowed ", authAction)
	}

	return authAction == "authorized"
}

func (auth *RedfishAuthorizationProperty) VerifyPrivilegeBits(requiredPrivs int) bool {
	// TODO: remove once the privlige information is being sent as an array of strings.
	Privileges := dellauth.PrivilegeBitsToStrings(requiredPrivs)
	return auth.VerifyPrivileges(Privileges)
}
