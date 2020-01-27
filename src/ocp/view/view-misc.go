package view

import (
	"context"
	"errors"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/model"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

// already locked at aggregate level when we get here
func (s *View) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error {
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

	// already have lock, used *Unlocked api
	modelObj := s.GetModelUnlocked(modelName)
	if modelObj == nil {
		log.MustLogger("GET").Debug("metadata specifies a nonexistent model name", "meta", meta, "view", s)
		return errors.New("metadata specifies a nonexistent model name")
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
			auth *domain.RedfishAuthorizationProperty,
			meta map[string]interface{},
		) error {
			property, ok := meta["property"].(string)
			if ok {
				if p, ok := m.GetPropertyOk(property); ok {
					rrp.ParseUnlocked(p)
					return nil
				}
			}
			return nil
		}
	}

	return formatterFn(ctx, s, modelObj, agg, rrp, auth, meta)
}

func (s *View) PropertyPatch(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	encopts *domain.NuEncOpts,
	meta map[string]interface{},
) error {

	s.Lock()
	defer s.Unlock()

	log.MustLogger("PATCH").Debug("PATCH START", "body", encopts.Parse, "meta", meta, "rrp", rrp)

	controllerName, ok := meta["controller"].(string)
	if !ok {
		log.MustLogger("PATCH").Debug("metadata is missing the controller name", "meta", meta)
		return xerrors.Errorf("metadata is missing the controller name: %v\n", meta)
	}

	controller, ok := s.controllers[controllerName]
	if !ok {
		log.MustLogger("PATCH").Debug("metadata specifies a nonexistent controller name", "meta", meta)
		return xerrors.Errorf("metadata specifies a nonexistent controller name: %v\n", meta)
	}

	property, ok := meta["property"].(string)
	if ok {
		newval, err := controller.UpdateRequest(ctx, property, encopts.Parse, auth)
		log.MustLogger("PATCH").Debug("update request", "newval", newval, "err", err)
		if e, ok := err.(domain.HTTP_code); ok {
			domain.AddEEMIMessage(encopts.HttpResponse, agg, "PATCHERROR", &e)

		} else if err == nil {
			rrp.Value = newval
			domain.AddEEMIMessage(encopts.HttpResponse, agg, "SUCCESS", nil)
		} else {
			log.MustLogger("PATCH").Debug("controller.UdpateRequest err is an unknown type", err)
			return xerrors.Errorf("controller.UdpateRequest err is an unknown type %t", err)
		}
		return nil

	}

	return errors.New("error updating: no property specified")
}
