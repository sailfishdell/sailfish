package view

import (
	"context"
	"errors"
	"fmt"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/model"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
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
		log.MustLogger("GET").Debug("metadata specifies a nonexistent model name", "meta", meta, "view", s)
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

	formatterFn(ctx, s, modelObj, rrp, meta)
}

func (s *View) PropertyPatch(
	ctx context.Context,
	rrp domain.RedfishResourceProperty,
	body interface{},
	meta map[string]interface{},
) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	log.MustLogger("PATCH").Debug("PATCH START", "body", body, "meta", meta, "rrp", rrp)

	controllerName, ok := meta["controller"].(string)
	if !ok {
		log.MustLogger("PATCH").Debug("metadata is missing the controller name", "meta", meta)
		return nil, errors.New(fmt.Sprintf("metadata is missing the controller name: %v\n", meta))
	}

	controller, ok := s.controllers[controllerName]
	if !ok {
		log.MustLogger("PATCH").Debug("metadata specifies a nonexistent controller name", "meta", meta)
		return nil, errors.New(fmt.Sprintf("metadata specifies a nonexistent controller name: %v\n", meta))
	}

	property, ok := meta["property"].(string)
	if ok {
		newval, err := controller.UpdateRequest(ctx, property, body)
		log.MustLogger("PATCH").Debug("update request", "newval", newval, "err", err)
		if err == nil {
			return newval, nil
		}
		return nil, errors.New("Error updating")
	}

	return nil, errors.New("Error updating: no property specified")
}
