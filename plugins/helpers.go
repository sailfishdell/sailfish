package plugins

import(
	"context"
	"fmt"
	"reflect"
	"sync"

	domain "github.com/superchalupa/go-redfish/redfishresource"
)

type Service struct {
	sync.Mutex
    Plugin domain.PluginType
}

func NewService() *Service {
    return &Service{}
}

func (s *Service) PluginType() domain.PluginType { return s.Plugin }


func RefreshProperty(
	ctx context.Context,
    s  interface{},
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
) {
	property, ok := meta["property"].(string)
	if ok {
		v := reflect.ValueOf(s)
		for i := 0; i < v.NumField(); i++ {
			// Get the field, returns https://golang.org/pkg/reflect/#StructField
			tag := v.Type().Field(i).Tag.Get("property")
			if tag == property {
				rrp.Value = v.Field(i).Interface()
				return
			}
		}
	}
	fmt.Printf("Incorrect metadata in aggregate: neither 'data' nor 'property' set to something handleable")
}
