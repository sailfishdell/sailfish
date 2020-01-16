package stdmeta

import (
	"context"
	"errors"
	"time"

	eh "github.com/looplab/eventhorizon"

	a "github.com/superchalupa/sailfish/src/dell-resources/attributedef"
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

func (s *SledProfile) PropertyPatch(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	encopts *domain.NuEncOpts,
	meta map[string]interface{},
) error {
	patch_timeout := 10
	//patch_timeout := 30000

	resURI := agg.ResourceURI

	v, err := domain.InstantiatePlugin(domain.PluginType(resURI))
	if err != nil || v == nil {
		return errors.New("could not find plugin for resource uri")
	}

	vw, ok := v.(*view.View)
	if !ok {
		return errors.New("could not typecast plugin as view")
	}

	mdl := vw.GetModel("default")
	if mdl == nil {
		return errors.New("could not find 'default' model in view")
	}

	sled_fqdd_raw, ok := mdl.GetPropertyOk("slot_contains")
	if !ok {
		return errors.New("could not get 'slot_contains' property from model")
	}

	sled_fqdd, ok := sled_fqdd_raw.(string)
	if !ok {
		return errors.New("could not typecast sled_fqdd into string")
	}

	reqUUID := eh.NewUUID()
	request_data := &a.AttributeUpdateRequestData{
		ReqID:         reqUUID,
		FQDD:          sled_fqdd,
		Group:         "Info",
		Index:         "1",
		Name:          "SledProfile",
		Value:         encopts.Parse,
		Authorization: *auth,
	}

	s.d.EventBus.PublishEvent(ctx, eh.NewEvent(a.AttributeUpdateRequest, request_data, time.Now()))

	tmctx, cancel := context.WithTimeout(ctx, time.Duration(patch_timeout)*time.Second)
	defer cancel()
	l, err := s.d.EventWaiter.Listen(tmctx, func(event eh.Event) bool {
		if event.EventType() != a.AttributeUpdated {
			return false
		}
		data, ok := event.Data().(*a.AttributeUpdatedData)
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
		return errors.New("failed to make attribute updated event listener")
	}
	l.Name = "sledprofile patch listener"
	defer l.Close()

	event, err := l.Wait(tmctx)
	if err != nil {
		return errors.New("timed out")
	} else {
		data, _ := event.Data().(*a.AttributeUpdatedData)

		hc := domain.HTTP_code{}
		if data.Error != "" {
			hc.Err_message = append(hc.Err_message, data.Error)
			domain.AddEEMIMessage(encopts.HttpResponse, agg, "PATCHERROR", &hc)
		} else {
			hc.Any_success = 1
			domain.AddEEMIMessage(encopts.HttpResponse, agg, "SUCCESS", &hc)
		}

	}
	return nil
}
