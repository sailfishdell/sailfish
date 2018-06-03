package generic_chassis

import (
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

func New(options ...model.Option) (*model.Model, error) {
	s := model.New()
	s.ApplyOption(options...)
	return s, nil
}

func WithUniqueName(uri string) model.Option {
	return model.PropertyOnce("unique_name", uri)
}

func AddManagedBy(odataID string) model.Option {
	return manageOdataIDList("managed_by", odataID)
}

// no locking because it's an Option, loc
func manageOdataIDList(name string, odataID string) model.Option {
	return func(s *model.Model) error {
		serversList, ok := s.GetPropertyOkUnlocked(name)
		if !ok {
			serversList = []map[string]string{}
		}
		sl, ok := serversList.([]map[string]string)
		if !ok {
			sl = []map[string]string{}
		}
		sl = append(sl, map[string]string{"@odata.id": odataID})

		s.UpdatePropertyUnlocked(name, sl)
		return nil
	}
}
