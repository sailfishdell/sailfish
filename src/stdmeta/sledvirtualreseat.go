package stdmeta

import (
	"context"
	"time"
  "regexp"
  "sync"
  "strings"

	eh "github.com/looplab/eventhorizon"
  eventpublisher "github.com/looplab/eventhorizon/publisher/local"

  "github.com/superchalupa/sailfish/src/log"
  "github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	"github.com/superchalupa/sailfish/src/ocp/view"
  "github.com/superchalupa/sailfish/src/actionhandler"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type Service struct {
  sync.RWMutex
  eb                  eh.EventBus
  logger              log.Logger
}

func SledVirtualReseatSvc(ctx context.Context, logger log.Logger, eb eh.EventBus) *Service {
  EventPublisher := eventpublisher.NewEventPublisher()
  eb.AddHandler(eh.MatchEvent(actionhandler.GenericActionEvent), EventPublisher)
  EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Sled Virtual Reseat Waiter"), eventwaiter.NoAutoRun)
  EventPublisher.AddObserver(EventWaiter)
  go EventWaiter.Run()

  sled_prefix := "System.Modular"
  virtual_reseat := "/Actions/Oem/DellChassis.VirtualReseat"

  ret := &Service{
    eb:                  eb,
    logger:              logger,
  }

  listener, err := EventWaiter.Listen(ctx, func(event eh.Event) bool {
    if event.EventType() == actionhandler.GenericActionEvent {
      return true
    }
    return false
  })
  if err != nil {
    return nil
  }

  go func() {
    defer listener.Close()
    for {
      select {
      case event := <-listener.Inbox():
        if e, ok := event.(syncEvent); ok {
          e.Done()
        }
        data, ok := event.Data().(*actionhandler.GenericActionEventData)
        if !ok {
          continue
        }
        uri := data.ResourceURI
        actionData, ok := data.ActionData.(map[string]string)
        if ok {
          _, ok1 := actionData["SledLoc"]
          _, ok2 := actionData["SledType"]
          if ok1 && ok2 {
            continue
          }
        }

        if !(strings.Contains(uri, sled_prefix) && strings.Contains(uri, virtual_reseat)) {
          continue
        }

        resURI := strings.Replace(uri, virtual_reseat, "", -1)
        attrURI := resURI + "/Attributes"

        attrv, err := domain.InstantiatePlugin(domain.PluginType(attrURI))
        if err != nil || attrv == nil {
          logger.Error("Could not find plugin for resource uri")
          continue
        }
        attrvw, ok := attrv.(*view.View)
        if !ok {
          logger.Error("Could not typecast plugin as view")
          continue
        }
        attrmdl := attrvw.GetModel("default")
        if attrmdl == nil {
          logger.Error("Could not find 'default' model in view")
          continue
        }

        all_attributes, ok := attrmdl.GetPropertyOk("attributes")
        if !ok {
          logger.Error("Could not get list of sled attributes")
          continue
        }

        flag := false
        var chassis_sub_type string
        all_attributes_map :=  all_attributes.(map[string]map[string]map[string]interface{})
        if val, ok := all_attributes_map["Info"]; ok {
          if val, ok := val["1"]; ok {
            if val, ok := val["ChassisSubType"]; ok {
              attribute_data := val.(attributes.AttributeData)
              chassis_sub_type = attribute_data.Value.(string)
              flag = true
            }
          }
        }
        if flag != true {
          logger.Error("Could not get sled type")
          continue
        }

        re := regexp.MustCompile(`System\.Modular\.(\d+).*`)
        matches := re.FindSubmatch([]byte(resURI))
        if len(matches) == 0 {
          logger.Error("Could not find sled slot number")
          continue
        }
        slot_num := string(matches[len(matches)-1])
        action_body := map[string]string{"SledType":chassis_sub_type,"SledLoc":slot_num}

        action_data := &actionhandler.GenericActionEventData{
          ID:          data.ID,
          CmdID:       data.CmdID,
          ResourceURI: uri,
          ActionData:  action_body,
          Method:      data.Method,
        }

        eb.PublishEvent(ctx, eh.NewEvent(actionhandler.GenericActionEvent, action_data, time.Now()))
      case <-ctx.Done():
        return
      }
    }
  }()

  return ret
}
