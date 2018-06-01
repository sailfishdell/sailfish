package list

import (
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

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
