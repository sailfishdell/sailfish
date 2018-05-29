package view

import (
	"context"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

//
//  PropertyGet vs GetProperty is confusing. Ooops. Should fix this naming snafu soon.
//

// already locked at aggregate level when we get here
func (s *View) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) {
	// but lock the actual service anyways, because we need to exclude anybody mucking with the backend directly. (side eye at you, viper)
	s.RLock()
	defer s.RUnlock()

	property, ok := meta["property"].(string)
	if ok {
		if p, ok := s.model.GetPropertyOk(property); ok {
			rrp.Value = p
			return
		}
	}
}
