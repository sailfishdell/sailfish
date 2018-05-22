package ec_manager

import (
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

type mapping struct {
	Property string
	FQDD     string
	Group    string
	Index    string
	Name     string
}

type service struct {
	*model.Service
}

func New(options ...interface{}) (*service, error) {
	s := &service{
		Service: model.NewService(),
	}
	// valid for consumer of this class to use without setting these, so put in a default
	s.UpdatePropertyUnlocked("bmc_manager_for_servers", []map[string]string{})
	s.UpdatePropertyUnlocked("bmc_manager_for_chassis", []map[string]string{})
	s.UpdatePropertyUnlocked("in_chassis", map[string]string{})

	s.ApplyOption(model.UUID())

	// user supplied options
	s.ApplyOption(options...)

	s.ApplyOption(model.PluginType(domain.PluginType("Managers/" + s.GetProperty("unique_name").(string))))
	s.ApplyOption(model.PropertyOnce("uri", "/redfish/v1/Managers/"+s.GetProperty("unique_name").(string)))
	return s, nil
}

func WithUniqueName(uri string) model.Option {
	return model.PropertyOnce("unique_name", uri)
}

func (s *service) GetUniqueName() string {
	return s.GetProperty("unique_name").(string)
}

type odataObj interface {
	GetProperty(string) interface{}
}

// no locking because it's an Option
func manageOdataIDList(name string, obj odataObj) Option {
	return func(s *service) error {

		// TODO: need to update @odata.count property, too

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

func AddManagerForChassis(obj odataObj) Option {
	return manageOdataIDList("bmc_manager_for_chassis", obj)
}

func (s *service) AddManagerForChassis(obj odataObj) {
	s.ApplyOption(AddManagerForChassis(obj))
}

func AddManagerForServer(obj odataObj) Option {
	return manageOdataIDList("bmc_manager_for_servers", obj)
}

func (s *service) AddManagerForServer(obj odataObj) {
	s.ApplyOption(AddManagerForServer(obj))
}

func InChassis(obj odataObj) Option {
	return func(s *service) error {
		s.UpdatePropertyUnlocked("in_chassis", map[string]string{"@odata.id": model.GetOdataID(obj)})
		return nil
	}
}

func (s *service) InChassis(obj odataObj) {
	s.ApplyOption(InChassis(obj))
}
