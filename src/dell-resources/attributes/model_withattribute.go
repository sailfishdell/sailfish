package attributes

import (
	"fmt"
	"github.com/superchalupa/sailfish/src/ocp/model"
)

// we can attributes to any model

//
// Use this to add an attribute or to update an attribute
//

// checks if the attribute sequence number is larger than what is stored in the model
// true - sequence number is larger, attribute can be updated
// false - sequence number is smaller, attribute should not be updated
func chk_seq(m *model.Model, seqProp string, seq int64) bool {
	v, ok := m.GetPropertyOkUnlocked(seqProp)
	if !ok || v == nil {
		m.UpdatePropertyUnlocked(seqProp, seq)
		return true
	}

	vint, ok := v.(int64)
	if !ok {
		return false
	}

	if seq >= vint {
		m.UpdatePropertyUnlocked(seqProp, seq)
		return true
	}
	return false
}

func WithAttribute(group, gindex, name string, value interface{}, seq int64) model.Option {
	return func(s *model.Model) error {
		var attributes map[string]map[string]map[string]interface{}

		seqProp := fmt.Sprintf("seq_%s%s%s", group, gindex, name)

		// special casing dumplog solution
		// TODO implemnet long term event sequence solution
		if group == "SupportAssist" && name == "Action" {
			ok := chk_seq(s, seqProp, seq)
			if !ok {
				return nil
			}
		}

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
