package bmc

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"
	"time"

	"github.com/superchalupa/go-redfish/src/log"
	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	ah "github.com/superchalupa/go-redfish/src/actionhandler"
)

const (
	BmcPlugin = domain.PluginType("obmc_bmc")
)

// OCP Profile Redfish BMC object

type service struct {
	*plugins.Service
}

func New(options ...interface{}) (*service, error) {
	s := &service{
		Service: plugins.NewService(plugins.PluginType(BmcPlugin)),
	}
	// valid for consumer of this class to use without setting these, so put in a default
	s.UpdatePropertyUnlocked("bmc_manager_for_servers", []map[string]string{})
	s.UpdatePropertyUnlocked("bmc_manager_for_chassis", []map[string]string{})
	s.UpdatePropertyUnlocked("in_chassis", map[string]string{})

	s.ApplyOption(plugins.UUID())
	s.ApplyOption(options...)
	s.ApplyOption(plugins.PropertyOnce("uri", "/redfish/v1/Managers/"+s.GetProperty("unique_name").(string)))
	return s, nil
}

func WithUniqueName(uri string) plugins.Option {
	return plugins.PropertyOnce("unique_name", uri)
}

type odataObj interface {
	GetOdataID() string
}

// no locking because it's an Option, loc
func manageOdataIDList(name string, obj odataObj) Option {
	return func(s *service) error {
		serversList, ok := s.GetPropertyOkUnlocked(name)
		if !ok {
			serversList = []map[string]string{}
		}
		sl, ok := serversList.([]map[string]string)
		if !ok {
			sl = []map[string]string{}
		}
		sl = append(sl, map[string]string{"@odata.id": obj.GetOdataID()})

		s.UpdatePropertyUnlocked(name, sl)
		return nil
	}
}

func AddManagerForChassis(obj odataObj) Option {
	return manageOdataIDList("bmc_manager_for_chassis", obj)
}

func (s *service) AddManagerForChassis(obj odataObj) {
	s.ApplyOption(AddManagerForChassis(obj))
}

func AddManagerForServer(obj odataObj) Option {
	return manageOdataIDList("bmc_manager_for_servers", obj)
}

func (s *service) AddManagerForServer(obj odataObj) {
	s.ApplyOption(AddManagerForServer(obj))
}

func InChassis(obj odataObj) Option {
	return func(s *service) error {
		s.UpdatePropertyUnlocked("in_chassis", map[string]string{"@odata.id": obj.GetOdataID()})
		return nil
	}
}

func (s *service) InChassis(obj odataObj) {
	s.ApplyOption(InChassis(obj))
}

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          s.GetUUID(),
			Collection:  false,
			ResourceURI: s.GetOdataID(),
			Type:        "#Manager.v1_1_0.Manager",
			Context:     "/redfish/v1/$metadata#Manager.Manager",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":                       s.GetProperty("unique_name"),
				"Name@meta":                s.Meta(plugins.PropGET("name")),
				"ManagerType":              "BMC",
				"Description@meta":         s.Meta(plugins.PropGET("description")),
				"Model@meta":               s.Meta(plugins.PropGET("model")),
				"DateTime@meta":            map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
				"DateTimeLocalOffset@meta": s.Meta(plugins.PropGET("timezone"), plugins.PropPATCH("timezone")),
				"FirmwareVersion@meta":     s.Meta(plugins.PropGET("version")),
				"Links": map[string]interface{}{
					"ManagerForServers@meta": s.Meta(plugins.PropGET("bmc_manager_for_servers")),
					"ManagerForChassis@meta": s.Meta(plugins.PropGET("bmc_manager_for_chassis")),
					"ManagerInChassis@meta":  s.Meta(plugins.PropGET("in_chassis")),
				},

				// Commented out until we figure out what these are supposed to be
				//"ServiceEntryPointUUID":    eh.NewUUID(),
				//"UUID":                     eh.NewUUID(),

				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"Actions": map[string]interface{}{
					"#Manager.Reset": map[string]interface{}{
						"target": s.GetOdataID() + "/Actions/Manager.Reset",
						"ResetType@Redfish.AllowableValues": []string{
							"ForceRestart",
							"GracefulRestart",
						},
					},
				},
			}})

	// The following redfish resource is created only for the purpose of being
	// a 'receiver' for the action command specified above.
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: s.GetOdataID() + "/Actions/Manager.Reset",
			Type:        "Action",
			Context:     "Action",
			Plugin:      "GenericActionHandler",
			Privileges: map[string]interface{}{
				"POST": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{},
		},
	)

	// stream processor for action events
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction(s.GetOdataID()+"/Actions/Manager.Reset")))
	if err != nil {
		log.MustLogger("ocp_bmc").Error("Failed to create event stream processor", "err", err)
		return
	}
	sp.RunForever(func(event eh.Event) {
		log.MustLogger("ocp_bmc").Info("Got action event", "event", event)

		eventData := domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"msg": "Not Implemented"},
			StatusCode: 500,
			Headers:    map[string]string{},
		}

		handler := s.GetProperty("manager.reset")
		if handler != nil {
			if fn, ok := handler.(func(eh.Event, *domain.HTTPCmdProcessedData)); ok {
				fn(event, &eventData)
			}
		}

		responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
		eb.PublishEvent(ctx, responseEvent)
	})
}
