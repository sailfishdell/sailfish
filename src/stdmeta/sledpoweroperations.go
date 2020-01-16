package stdmeta

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	a "github.com/superchalupa/sailfish/src/dell-resources/attributedef"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type Service struct {
	sync.RWMutex
	eb     eh.EventBus
	logger log.Logger
}

type BusObjs interface {
	GetWaiter() *eventwaiter.EventWaiter
	GetBus() eh.EventBus
}

func SledPwrOperationsSvc(ctx context.Context, logger log.Logger, d BusObjs) *Service {
	sled_prefix := "System.Modular"
	virtualReseat := "/Actions/Oem/DellChassis.VirtualReseat"
	idracReset := "/Actions/Oem/DellChassis.iDRACReset"

	re := regexp.MustCompile(`System\.Modular\.(\d+).*`)
	ret := &Service{
		eb:     d.GetBus(),
		logger: logger,
	}

	listener := eventwaiter.NewListener(ctx, logger, d.GetWaiter(), func(event eh.Event) bool {
		return event.EventType() == actionhandler.GenericActionEvent
	})
	// no listener.Close(), this runs forever

	go listener.ProcessEvents(ctx, func(event eh.Event) {
		data, ok := event.Data().(*actionhandler.GenericActionEventData)
		if !ok {
			return
		}
		uri := data.ResourceURI
		actionData, ok := data.ActionData.(map[string]string)
		if ok {
			_, ok1 := actionData["SledLoc"]
			_, ok2 := actionData["SledType"]
			if ok1 && ok2 {
				return
			}
		}

		var resURI string
		if strings.Contains(uri, sled_prefix) && strings.Contains(uri, virtualReseat) {
			resURI = strings.Replace(uri, virtualReseat, "", -1)
		} else if strings.Contains(uri, sled_prefix) && strings.Contains(uri, idracReset) {
			resURI = strings.Replace(uri, idracReset, "", -1)
		} else {
			return
		}

		attrURI := resURI + "/Attributes"

		attrv, err := domain.InstantiatePlugin(domain.PluginType(attrURI))
		if err != nil || attrv == nil {
			logger.Error("SledPower: Could not find plugin for resource uri")
			return
		}
		attrvw, ok := attrv.(*view.View)
		if !ok {
			logger.Error("SledPower:Could not typecast plugin as view")
			return
		}
		attrmdl := attrvw.GetModel("default")
		if attrmdl == nil {
			logger.Error("SledPower: Could not find 'default' model in view")
			return
		}

		all_attributes, ok := attrmdl.GetPropertyOk("attributes")
		if !ok {
			logger.Error("SledPower: Could not get list of sled attributes")
			return
		}

		flag := false
		var chassis_sub_type string
		all_attributes_map := all_attributes.(map[string]map[string]map[string]interface{})
		if val, ok := all_attributes_map["Info"]; ok {
			if val, ok := val["1"]; ok {
				if val, ok := val["ChassisSubType"]; ok {
					attribute_data := val.(a.AttributeData)
					chassis_sub_type = attribute_data.Value.(string)
					flag = true
				}
			}
		}
		if !flag {
			logger.Error("SledPower: Could not get sled type")
			return
		}

		matches := re.FindSubmatch([]byte(resURI))
		if len(matches) == 0 {
			logger.Error("SledPower: Could not find sled slot number")
			return
		}
		slot_num := string(matches[len(matches)-1])
		action_body := map[string]string{"SledType": chassis_sub_type, "SledLoc": slot_num}

		action_data := &actionhandler.GenericActionEventData{
			ID:          data.ID,
			CmdID:       data.CmdID,
			ResourceURI: uri,
			ActionData:  action_body,
			Method:      data.Method,
		}

		ret.eb.PublishEvent(ctx, eh.NewEvent(actionhandler.GenericActionEvent, action_data, time.Now()))
	})

	return ret
}
