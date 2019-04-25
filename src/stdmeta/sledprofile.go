package stdmeta

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

var (
	SledProfilePlugin = domain.PluginType("SledProfile")
)

func SetupSledProfilePlugin(d *domain.DomainObjects) {
	domain.RegisterPlugin(func() domain.Plugin { return &SledProfile{d: d} })
}

type SledProfile struct {
	d *domain.DomainObjects
}

func (s *SledProfile) PluginType() domain.PluginType { return SledProfilePlugin }

type syncEvent interface {
	Add(int)
	Done()
}

func (s *SledProfile) PropertyPatch(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	encopts interface{},
	meta map[string]interface{},
) error {
	patch_timeout := 10
	//patch_timeout := 30000
	any_success := 0

	resURI := agg.ResourceURI

	v, err := domain.InstantiatePlugin(domain.PluginType(resURI))
	if err != nil || v == nil {
		return errors.New("Could not find plugin for resource uri")
	}

	vw, ok := v.(*view.View)
	if !ok {
		return errors.New("Could not typecast plugin as view")
	}

	mdl := vw.GetModel("default")
	if mdl == nil {
		return errors.New("Could not find 'default' model in view")
	}

	sled_fqdd_raw, ok := mdl.GetPropertyOk("slot_contains")
	if !ok {
		return errors.New("Could not get 'slot_contains' property from model")
	}

	sled_fqdd, ok := sled_fqdd_raw.(string)
	if !ok {
		return errors.New("Could not typecast sled_fqdd into string")
	}

	reqUUID := eh.NewUUID()
	request_data := &attributes.AttributeUpdateRequestData{
		ReqID:         reqUUID,
		FQDD:          sled_fqdd,
		Group:         "Info",
		Index:         "1",
		Name:          "SledProfile",
		Value:         encopts,
		Authorization: *auth,
	}

	s.d.EventBus.PublishEvent(ctx, eh.NewEvent(attributes.AttributeUpdateRequest, request_data, time.Now()))

	tmctx, _ := context.WithTimeout(ctx, time.Duration(patch_timeout)*time.Second)
	l, err := s.d.EventWaiter.Listen(tmctx, func(event eh.Event) bool {
		if event.EventType() != attributes.AttributeUpdated {
			return false
		}
		data, ok := event.Data().(*attributes.AttributeUpdatedData)
		if !ok {
			return false
		}
		if data.Name != "SledProfile" {
			return false
		}
		if data.ReqID != reqUUID {
			return false
		}
		return true
	})
	if err != nil {
		return errors.New("Failed to make attribute updated event listener")
	}
	l.Name = "sledprofile patch listener"
	defer l.Close()

	event, err := l.Wait(tmctx)
	if err != nil {
		return errors.New("TIMED OUT")
	} else {
		data, _ := event.Data().(*attributes.AttributeUpdatedData)
		err_extendedinfos := []interface{}{}

		if data.Error != "" {
			msg := domain.ExtendedInfo{}
			err := json.Unmarshal([]byte(data.Error), &msg)
			if err != nil {
				return errors.New("Error updating: Could not unmarshal EEMI message")
			}
			err_extendedinfos = append(err_extendedinfos, msg)
		} else {
			any_success = 1
		}

		if any_success > 0 {
			default_msg := domain.ExtendedInfo{}
			oeim := *domain.NewObjectExtendedInfoMessages([]interface{}{default_msg.GetDefaultExtendedInfo()})
			return &domain.CombinedPropObjInfoError{
				ObjectExtendedInfoMessages: oeim,
				NumSuccess:                 any_success,
			}
		} else {
			oeem := *domain.NewObjectExtendedErrorMessages(err_extendedinfos)
			return &domain.CombinedPropObjInfoError{
				ObjectExtendedErrorMessages: oeem,
				NumSuccess:                  any_success,
			}
		}
	}
	return nil
}
