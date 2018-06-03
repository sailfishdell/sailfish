package attribute_property

import (
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

// we can attributes to any model

//
// Use this to add an attribute or to update an attribute
//
func WithAttribute(group, gindex, name string, value interface{}) model.Option {
	return func(s *model.Model) error {
		var attributes map[string]map[string]map[string]interface{}

		attributesRaw, ok := s.GetPropertyOkUnlocked("attributes")
		if ok {
			attributes, ok = attributesRaw.(map[string]map[string]map[string]interface{})
		}

		if !ok {
			attributes = map[string]map[string]map[string]interface{}{}
		}

		groupMap, ok := attributes[group]
		if !ok {
			groupMap = map[string]map[string]interface{}{}
			attributes[group] = groupMap
		}

		index, ok := groupMap[gindex]
		if !ok {
			index = map[string]interface{}{}
			groupMap[gindex] = index
		}

		index[name] = value

		s.UpdatePropertyUnlocked("attributes", attributes)

		return nil
	}
}
