package view

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

// already locked at aggregate level when we get here
func (s *View) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) {
	// but lock the actual service anyways, because we need to exclude anybody
	// mucking with the backend directly. (side eye at you, viper)
	s.RLock()
	defer s.RUnlock()

	modelRaw, ok := meta["model"]
	if !ok {
		modelRaw = "default"
	}

	modelName, ok := modelRaw.(string)
	if !ok {
		modelName = "default"
	}

	modelObj := s.GetModel(modelName)
	if modelObj == nil {
		log.MustLogger("GET").Debug("metadata specifies a nonexistent model name", "meta", meta)
		return
	}

	formatterRaw, ok := meta["formatter"]
	if !ok {
		formatterRaw = "default"
	}

	formatterName, ok := formatterRaw.(string)
	if !ok {
		formatterName = "default"
	}

	formatterFn, ok := s.outputFormatters[formatterName]
	if !ok {
		// default "raw" formatter
		formatterFn = func(
			ctx context.Context,
			v *View,
			m *model.Model,
			agg *domain.RedfishResourceAggregate,
			rrp *domain.RedfishResourceProperty,
			meta map[string]interface{},
		) error {
			property, ok := meta["property"].(string)
			if ok {
				if p, ok := m.GetPropertyOk(property); ok {
					rrp.Value = p
					return nil
				}
			}
			return nil
		}
	}

	formatterFn(ctx, s, modelObj, agg, rrp, meta)
}

func (s *View) PropertyPatch(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
	body interface{},
	present bool,
) {
	s.Lock()
	defer s.Unlock()

	log.MustLogger("PATCH").Debug("PATCH START", "body", body, "present", present, "meta", meta, "rrp", rrp)

	controllerName, ok := meta["controller"].(string)
	if !ok {
		log.MustLogger("PATCH").Debug("metadata is missing the controller name", "meta", meta)
		return
	}

	controller, ok := s.controllers[controllerName]
	if !ok {
		log.MustLogger("PATCH").Debug("metadata specifies a nonexistent controller name", "meta", meta)
		return
	}

	if present {
		property, ok := meta["property"].(string)
		if ok {
			newval, err := controller.UpdateRequest(ctx, property, body)
			log.MustLogger("PATCH").Debug("update request", "newval", newval, "err", err)
			if err == nil {
				rrp.Value = newval
			}
			return
		}
	}
}
