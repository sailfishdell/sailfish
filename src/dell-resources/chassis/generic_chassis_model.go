package generic_chassis

import (
	"fmt"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func New(options ...plugins.Option) (*plugins.Service, error) {
	s := plugins.NewService()

	s.ApplyOption(plugins.UUID())
	s.ApplyOption(options...)
	s.ApplyOption(plugins.PropertyOnce("uri", "/redfish/v1/Chassis/"+s.GetProperty("unique_name").(string)))
	s.ApplyOption(plugins.PluginType(domain.PluginType("attribute property: " + fmt.Sprintf("Chassis/%s", s.GetProperty("unique_name")))))
	return s, nil
}

func WithUniqueName(uri string) plugins.Option {
	return plugins.PropertyOnce("unique_name", uri)
}

type odataObj interface {
	GetOdataID() string
}

func AddManagedBy(obj odataObj) plugins.Option {
	return manageOdataIDList("managed_by", obj)
}

// no locking because it's an Option, loc
func manageOdataIDList(name string, obj odataObj) plugins.Option {
	return func(s *plugins.Service) error {
		serversList, ok := s.GetPropertyOkUnlocked(name)
		if !ok {
			serversList = []map[string]string{}
		}
		sl, ok := serversList.([]map[string]string)
		if !ok {
			sl = []map[string]string{}
		}
		sl = append(sl, map[string]string{"@odata.id": obj.GetOdataID()})

		s.UpdatePropertyUnlocked(name, sl)
		return nil
	}
}
