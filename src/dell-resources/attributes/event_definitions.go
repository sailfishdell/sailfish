package attributes

import (
	eh "github.com/looplab/eventhorizon"
	"github.com/mitchellh/mapstructure"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

const (
	AttributeUpdated                eh.EventType = "AttributeUpdated"
	AttributeUpdateRequest          eh.EventType = "AttributeUpdateRequest"
	AttributeGetCurrentValueRequest eh.EventType = "AttributeGetCurrentValueRequest"
)

func init() {
	eh.RegisterEventData(AttributeUpdated, func() eh.EventData { return &AttributeUpdatedData{} })
	eh.RegisterEventData(AttributeUpdateRequest, func() eh.EventData { return &AttributeUpdateRequestData{} })
	eh.RegisterEventData(AttributeGetCurrentValueRequest, func() eh.EventData { return &AttributeGetCurrentValueRequestData{} })
}

type PrivilegeData struct {
	License        int
	ReadPrivilege  int
	WritePrivilege int
	Readonly       bool
	IsSuppressed   bool
	Private        bool
}

type AttributeData struct {
	Privileges PrivilegeData
	Value      interface{}
}

type AttributeUpdatedData struct {
	Privileges PrivilegeData
	ReqID      eh.UUID
	FQDD       string
	Group      string
	Index      string
	Name       string
	Value      interface{}
	Error      string
	Event_seq  int64
}

type AttributeUpdateRequestData struct {
	ReqID         eh.UUID
	FQDD          string
	Group         string
	Index         string
	Name          string
	Value         interface{}
	Authorization domain.RedfishAuthorizationProperty
}

type AttributeGetCurrentValueRequestData struct {
	FQDD  string
	Group string
	Index string
	Name  string
}

func (ad *AttributeData) Valid(attrVal interface{}) bool {
	err := mapstructure.Decode(attrVal, ad)
	if err != nil {
		return false
	}
	return true
}

func (ad *AttributeData) WriteAllowed(attrVal interface{}, auth *domain.RedfishAuthorizationProperty) bool {
	switch attrVal.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr, string, float32, float64:
		// for patch on Chassis/X
	default:
		// for patch on Chassis/X/Attributes
		if !ad.Valid(attrVal) {
			return false
		}
	}

	if ad.Privileges.Private ||
		ad.Privileges.Readonly ||
		!auth.VerifyPrivilegeBits(ad.Privileges.WritePrivilege) {
		//fmt.Println("not allowed to write ", ad)
		return false
	}
	return true
}

func (ad *AttributeData) ReadAllowed(attrVal interface{}, auth *domain.RedfishAuthorizationProperty) bool {
	if !ad.Valid(attrVal) {
		return false
	}

	if ad.Privileges.Private ||
		ad.Privileges.IsSuppressed ||
		!auth.VerifyPrivilegeBits(ad.Privileges.ReadPrivilege) {
		//fmt.Println("not allowed to read ", ad)
		return false
	}
	return true
}
