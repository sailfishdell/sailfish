package view

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/model"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type isHTTPCode interface{
  ErrMessage() []string
  AnySuccess() int
}

// already locked at aggregate level when we get here
func (s *View) PropertyGet(
	ctx context.Context,
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

	modelObj := s.GetModel(modelName)
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

	return formatterFn(ctx, s, modelObj, rrp, auth, meta)
}

func (s *View) PropertyPatch(
	ctx context.Context,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	body interface{},
	meta map[string]interface{},
) error {

	s.Lock()
	defer s.Unlock()

	log.MustLogger("PATCH").Debug("PATCH START", "body", body, "meta", meta, "rrp", rrp)

	controllerName, ok := meta["controller"].(string)
	if !ok {
		log.MustLogger("PATCH").Debug("metadata is missing the controller name", "meta", meta)
		return errors.New(fmt.Sprintf("metadata is missing the controller name: %v\n", meta))
	}

	controller, ok := s.controllers[controllerName]
	if !ok {
		log.MustLogger("PATCH").Debug("metadata specifies a nonexistent controller name", "meta", meta)
		return errors.New(fmt.Sprintf("metadata specifies a nonexistent controller name: %v\n", meta))
	}

	property, ok := meta["property"].(string)
	if ok {
		newval, err := controller.UpdateRequest(ctx, property, body, auth)
		log.MustLogger("PATCH").Debug("update request", "newval", newval, "err", err)
    if e, ok := err.(isHTTPCode); ok {
      any_success := e.AnySuccess()
      //errors reported from patch & formatted correctly
      err_extendedinfos := []interface{}{}
      for _, err_msg := range(e.ErrMessage()) {
        //generted extended error info msg for each err
        //de-serialize err_msg here! need to turn from string into map[string]interface{}
        msg := domain.ExtendedInfo{}
        err := json.Unmarshal([]byte(err_msg), &msg)
        if err != nil {
          log.MustLogger("PATCH").Crit("Error could not be unmarshalled to an EEMI message")
          return errors.New("Error updating: Could not unmarshal EEMI message")
        }
        err_extendedinfos = append(err_extendedinfos, msg)
      }
      oeem := *domain.NewObjectExtendedErrorMessages(err_extendedinfos)
      num_success := *domain.NewNumSuccess([]int{any_success})
      return &domain.CombinedPropObjInfoError{
        ObjectExtendedErrorMessages: oeem,
        NumSuccess: num_success,
      }
    } else if err == nil {
			rrp.Value = newval
      default_msg := domain.ExtendedInfo{}
      oeim := *domain.NewObjectExtendedInfoMessages([]interface{}{default_msg.GetDefaultExtendedInfo()})
      num_success := *domain.NewNumSuccess([]int{1})
			return &domain.CombinedPropObjInfoError{
				ObjectExtendedInfoMessages:   oeim,
        NumSuccess: num_success,
			}
		}

		return errors.New("Error updating: patch error message not formatted properly")
	}

	return errors.New("Error updating: no property specified")
}
