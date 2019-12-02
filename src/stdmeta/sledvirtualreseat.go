package stdmeta

import (
	"context"
	"regexp"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	a "github.com/superchalupa/sailfish/src/dell-resources/attributedef"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type dispatcher interface {
	AddEventHandler(string, eh.EventType, func(event eh.Event))
}

func SledVirtualReseatSvc(ctx context.Context, logger log.Logger, am3Svc dispatcher, d *domain.DomainObjects) {
	sled_prefix := "System.Modular"
	virtual_reseat := "/Actions/Oem/DellChassis.VirtualReseat"
	eb := d.EventBus

	am3Svc.AddEventHandler("virtualReseatHandler", actionhandler.GenericActionEvent, func(event eh.Event) {
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

		if !(strings.Contains(uri, sled_prefix) && strings.Contains(uri, virtual_reseat)) {
			return
		}

		resURI := strings.Replace(uri, virtual_reseat, "", -1)
		attrURI := resURI + "/Attributes"

		attrv, err := domain.InstantiatePlugin(domain.PluginType(attrURI))
		if err != nil || attrv == nil {
			logger.Error("Could not find plugin for resource uri")
			return
		}
		attrvw, ok := attrv.(*view.View)
		if !ok {
			logger.Error("Could not typecast plugin as view")
			return
		}
		attrmdl := attrvw.GetModel("default")
		if attrmdl == nil {
			logger.Error("Could not find 'default' model in view")
			return
		}

		all_attributes, ok := attrmdl.GetPropertyOk("attributes")
		if !ok {
			logger.Error("Could not get list of sled attributes")
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
		if flag != true {
			logger.Error("Could not get sled type")
			return
		}

		re := regexp.MustCompile(`System\.Modular\.(\d+).*`)
		matches := re.FindSubmatch([]byte(resURI))
		if len(matches) == 0 {
			logger.Error("Could not find sled slot number")
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

		eb.PublishEvent(ctx, eh.NewEvent(actionhandler.GenericActionEvent, action_data, time.Now()))
	})
}
