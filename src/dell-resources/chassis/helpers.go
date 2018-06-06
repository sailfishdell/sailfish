package helpers

// this is a terrible, horrible, no-good name for a package. Lets fix it.

import (
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

// no locking because it's an Option, loc
func AppendOdataIDList(name string, odataID string) model.Option {
	return func(s *model.Model) error {
		idList, ok := s.GetPropertyOkUnlocked(name)
		if !ok {
			idList = []map[string]string{}
		}
		sl, ok := idList.([]map[string]string)
		if !ok {
			sl = []map[string]string{}
		}
		sl = append(sl, map[string]string{"@odata.id": odataID})

		s.UpdatePropertyUnlocked(name, sl)
		return nil
	}
}
