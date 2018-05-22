package generic_chassis

import (
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func New(options ...model.Option) (*model.Model, error) {
	s := model.NewModel()

	s.ApplyOption(model.UUID())
	s.ApplyOption(options...)
	s.ApplyOption(model.PropertyOnce("uri", "/redfish/v1/Chassis/"+s.GetProperty("unique_name").(string)))
	s.ApplyOption(model.PluginType(domain.PluginType("Chassis/" + s.GetProperty("unique_name").(string))))
	return s, nil
}

func WithUniqueName(uri string) model.Option {
	return model.PropertyOnce("unique_name", uri)
}

type odataObj interface {
	GetProperty(string) interface{}
}

func AddManagedBy(obj odataObj) model.Option {
	return manageOdataIDList("managed_by", obj)
}

// no locking because it's an Option, loc
func manageOdataIDList(name string, obj odataObj) model.Option {
	return func(s *model.Model) error {
		serversList, ok := s.GetPropertyOkUnlocked(name)
		if !ok {
			serversList = []map[string]string{}
		}
		sl, ok := serversList.([]map[string]string)
		if !ok {
			sl = []map[string]string{}
		}
		sl = append(sl, map[string]string{"@odata.id": model.GetOdataID(obj)})

		s.UpdatePropertyUnlocked(name, sl)
		return nil
	}
}
