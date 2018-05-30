package ec_manager

import (
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

func New(options ...model.Option) (*model.Model, error) {
	s := model.NewModel()

	// valid for consumer of this class to use without setting these, so put in a default
	s.UpdatePropertyUnlocked("bmc_manager_for_servers", []map[string]string{})
	s.UpdatePropertyUnlocked("bmc_manager_for_chassis", []map[string]string{})
	s.UpdatePropertyUnlocked("in_chassis", map[string]string{})

	// user supplied options
	s.ApplyOption(options...)

    s.ApplyOption(model.PropertyOnce("uri", "/redfish/v1/Managers/"+s.GetProperty("unique_name").(string)))
	return s, nil
}

func WithUniqueName(uri string) model.Option {
	return model.PropertyOnce("unique_name", uri)
}

type odataObj interface {
	GetProperty(string) interface{}
}

// no locking because it's an Option
func manageOdataIDList(name string, obj odataObj) model.Option {
	return func(s *model.Model) error {

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

func AddManagerForChassis(obj odataObj) model.Option {
	return manageOdataIDList("bmc_manager_for_chassis", obj)
}

func AddManagerForServer(obj odataObj) model.Option {
	return manageOdataIDList("bmc_manager_for_servers", obj)
}

func InChassis(obj odataObj) model.Option {
	return func(s *model.Model) error {
		s.UpdatePropertyUnlocked("in_chassis", map[string]string{"@odata.id": model.GetOdataID(obj)})
		return nil
	}
}
