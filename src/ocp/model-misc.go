package plugins

import (
	"context"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

//
//  PropertyGet vs GetProperty is confusing. Ooops. Should fix this naming snafu soon.
//

// already locked at aggregate level when we get here
func (s *Service) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) {
	// but lock the actual service anyways, because we need to exclude anybody mucking with the backend directly. (side eye at you, viper)
	s.RLock()
	defer s.RUnlock()

	property, ok := meta["property"].(string)
	if ok {
		if p, ok := s.properties[property]; ok {
			rrp.Value = p
			return
		}
	}
}

// already locked at aggregate level when we get here
func (s *Service) PropertyPatch(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
	body interface{},
	present bool,
) {
	// but lock the actual service anyways, because we need to exclude anybody mucking with the backend directly. (side eye at you, viper)
	s.Lock()
	defer s.Unlock()

	property, ok := meta["property"].(string)
	if present && ok {
		validator, ok := s.properties[property+"@meta.validator"]
		if ok {
			if vFN, ok := validator.(func(*domain.RedfishResourceProperty, interface{})); ok {
				vFN(rrp, body)
			}
		}

		// either of above nested if()'s fail, we end up here:
		if !ok {
			// validator function can coerce type, act as a notification callback, or enforce constraints
			s.properties[property] = body
			rrp.Value = body
		}

		// notify anybody that cares
		callback, ok := s.properties[property+"@meta.callback"]
		if ok {
			if cb, ok := callback.([]func(interface{})); ok {
				for _, fn := range cb {
					fn(rrp.Value)
				}
			}
		}
	}
}

// the observer will be called after the property is set from the web interface
func (s *Service) AddPropertyObserver(property string, fn func(interface{})) {
	cbListInt, ok := s.properties[property+"@meta.callback"]
	if !ok {
		cbListInt = []func(interface{}){}
	}
	cbList, ok := cbListInt.([]func(interface{}))
	if !ok {
		cbListInt = []func(interface{}){}
	}
	cbList = append(cbList, fn)
	s.properties[property+"@meta.callback"] = cbList
}
