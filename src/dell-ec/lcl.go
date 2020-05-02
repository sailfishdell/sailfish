package dell_ec

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

// Noncompliant FQDDs: RCPUSB, RSPI, RTC, ControlPanel, QuickSync, LCD, LED, Frontpanel
// As there is no plan to add actual tree paths for noncompliant URIs, noncompliant FQDDs
// will return the System.Chassis.1 URI.
// If they are added, frontpanel FQDDS will have to check the message for key words to
// determine the correct origin paths
func link_mapper(fqdd string) string {
	// All FQDD tree paths branch from /redfish/v1/Chassis
	ret_string := "/redfish/v1/Chassis/"

	chassis_subparts := []string{`IOM\.Slot`, `System\.Modular`, `iDRAC\.Embedded`, `CMC\.Integrated`, `Fan\.Slot`, `PSU\.Slot`, `Temperature\.NODE_AMBIENT`, `Temperature\.CHASSIS_AMBIENT`, `System\.Chassis`}
	FQDD_parts := strings.Split(fqdd, "#")

	for i, _ := range FQDD_parts {
		// Let right-most FQDD part take precedence, look for matching subparts
		FQDD_part := FQDD_parts[len(FQDD_parts)-1-i]
		for _, k := range chassis_subparts {
			// Find matching chassis subpart with any slot numbers
			re := regexp.MustCompile(k + `\.*\w*`)
			matched := re.Find([]byte(FQDD_part))
			if matched == nil {
				continue
			}
			FQDD_matched := string(matched)

			// Substitute matches for corrected versions
			if FQDD_matched == "CMC.Integrated.0" {
				FQDD_matched = "CMC.Integrated.1"
			} else if k == `iDRAC\.Embedded` {
				FQDD_matched = strings.Replace(FQDD_matched, "iDRAC.Embedded", "System.Modular", -1)
			}

			// Fans, PSUs, and temperatures have specific paths off System.Chassis.1
			if k == `Fan\.Slot` {
				ret_string += "System.Chassis.1/Sensors/Fans/" + FQDD_matched
			} else if k == `PSU\.Slot` {
				ret_string += "System.Chassis.1/Sensors/PowerSupplies/" + FQDD_matched
			} else if k == `Temperature\.NODE_AMBIENT` || k == `Temperature\.CHASSIS_AMBIENT` {
				ret_string += "System.Chassis.1/Sensors/Temperatures/System.Chassis.1%23" + FQDD_matched
			} else {
				ret_string += FQDD_matched
			}

			// If a matching subpart with valid tree path is found, return
			goto early_out
		}
	}
	// For all other FQDDs, default to System.Chassis.1 path (including all noncompliant URIs)
	ret_string += "System.Chassis.1"

early_out:
	return ret_string
}

func initLCL(logger log.Logger, instantiateSvc *testaggregate.Service, am3Svc am3.Service, ch eh.CommandHandler, d *domain.DomainObjects) {
	awesome_mapper2.AddFunction("health_alert", func(args ...interface{}) (interface{}, error) {
		ss, ok := args[0].(string)
		if !ok {
			logger.Crit("Mapper configuration error: subsystem %s in not a string", "args[0]")
			return nil, errors.New("mapper configuration error: subsystem is not a string")
		}

		health, ok := args[1].(string)
		if !ok {
			logger.Crit("Mapper configuration error: health %s in not a string", "args[0]")
			return nil, errors.New("mapper configuration error: health is not a string")
		}

		t := time.Now()
		cTime := t.Format("2006-01-02T15:04:05-07:00")
		ma := []string{health}

		//Create Alert type event:
		d.EventBus.PublishEvent(context.Background(),
			eh.NewEvent(eventservice.RedfishEvent, &eventservice.RedfishEventData{
				EventType:         "Alert",
				EventId:           "1",
				EventTimestamp:    cTime,
				Severity:          "Informational",
				Message:           "The chassis health is " + health,
				MessageId:         "CMC8550",
				MessageArgs:       ma,
				OriginOfCondition: ss,
			}, time.Now()))

		return true, nil
	})

	var syncModels func(m *model.Model, updates []model.Update)
	type newfirm struct {
		uri  string
		mdls []*model.Model
	}
	newchan := make(chan newfirm, 90)
	trigger := make(chan struct{})
	firmwareInventoryViews := map[string]*view.View{}

	am3Svc.AddEventHandler("AddSwinv", domain.RedfishResourceCreated, func(event eh.Event) {
		data, ok := event.Data().(*domain.RedfishResourceCreatedData)
		if !ok {
			logger.Error("Redfish Resource Created event did not match", "type", event.EventType, "data", event.Data())
			return
		}

		resourceURI := format_uri(data.ResourceURI)

		v, err := domain.InstantiatePlugin(domain.PluginType(resourceURI))
		if err != nil || v == nil {
			return
		}

		vw, ok := v.(*view.View)
		if !ok {
			return
		}

		mdl := vw.GetModel("swinv")
		if mdl == nil {
			return
		}

		mdlMap := vw.GetModels("swinv")
		if len(mdlMap) == 0 {
			return
		}

		mdls := []*model.Model{}

		for mdlName, mdl := range mdlMap {
			mdl.AddObserver(mdlName, syncModels)
			mdls = append(mdls, mdl)
		}

		newchan <- newfirm{resourceURI, mdls}
	})

	syncModels = func(m *model.Model, updates []model.Update) {
		select {
		case trigger <- struct{}{}:
		default:
		}
	}

	go func() {
		swinvList := map[string][]*model.Model{}
		uris_in_use := map[string]bool{}

		for {

			// Wait for this thread to be kicked
			// either a model gets updated (trigger)
			// or a new model is added (newchan)
			select {
			case <-trigger:
			case n := <-newchan:
				_, ok := swinvList[n.uri]
				if ok {
					swinvList[n.uri] = append(swinvList[n.uri], n.mdls...)
				} else {
					swinvList[n.uri] = n.mdls
				}
				continue
			}

			fqdd_mappings := map[string][]string{}
			uri_mappings := map[string][]string{}

			// reset all URIs to be "not in use". If they are still in use, then
			// they will be marked "true" while each model is being scanned. Any
			// URIs which are still marked as "false" at the end will be deleted.
			set_all_in_map_bool(uris_in_use, false)

			// scan through each model and build our new inventory uris
			// need to iterate through models.  With the same work flow below.. When iteration is complete... need to add uris and fqdds to model.
			for uri, mdls := range swinvList {
				for _, mdl := range mdls {
					fqddRaw, ok := mdl.GetPropertyOk("fw_fqdd")
					if !ok || fqddRaw == nil {
						logger.Debug("swinv DID NOT GET fqdd raw with " + uri)
						continue
					}

					fqdd, ok := fqddRaw.(string)
					if !ok || fqdd == "" || fqdd == "unknown" {
						logger.Debug("swinv DID NOT GET fqdd string with " + uri)
						continue
					}

					classRaw, ok := mdl.GetPropertyOk("fw_device_class")
					if !ok || classRaw == nil {
						logger.Debug("swinv DID NOT GET device_class raw with " + uri)
						continue
					}

					class, ok := classRaw.(string)
					if !ok || class == "" || class == "unknown" {
						logger.Debug("swinv DID NOT GET class string with " + uri)
						continue
					}

					versionRaw, ok := mdl.GetPropertyOk("fw_version")
					if !ok || versionRaw == nil {
						logger.Debug("swinv DID NOT GET version raw with " + uri)
						continue
					}

					version, ok := versionRaw.(string)
					if !ok || version == "" || version == "unknown" {
						logger.Debug("swinv DID NOT GET version string with " + uri)
						continue
					}

					compVerTuple := "Installed-" + class + "-" + version

					updateableRaw, ok := mdl.GetPropertyOk("fw_updateable")
					if !ok {
						logger.Debug("swinv DID NOT GET updateable string with " + uri)
						continue
					}

					updateable := false
					updateableStr, _ := updateableRaw.(string)
					if strings.EqualFold(updateableStr, "Yes") || strings.EqualFold(updateableStr, "True") {
						updateable = true
					} else if updateableStr == "unknown" {
						logger.Debug("swinv DID NOT GET a good value for updateablestr " + uri)
						continue
					}

					installDateRaw, ok := mdl.GetPropertyOk("fw_install_date")
					if !ok {
						logger.Debug("swinv DID NOT GET install date string with " + uri)
						continue
					}

					installDate, ok := installDateRaw.(string)
					if !ok || installDate == "unknown" {
						logger.Debug("swinv DID NOT GET class string with " + uri)
						continue
					} else if installDate == "" {
						logger.Debug("swinv DID NOT GET class string with " + uri)
						installDate = "1970-01-01T00:00:00Z"

					}

					nameRaw, ok := mdl.GetPropertyOk("fw_name")
					if !ok {
						logger.Debug("swinv DID NOT GET name string with " + uri)
						continue
					}

					name, ok := nameRaw.(string)
					if !ok || name == "" || name == "unknown" {
						logger.Debug("swinv DID NOT GET name string with " + uri)
						continue
					}

					if _, ok := firmwareInventoryViews[compVerTuple]; !ok {
						_, vw, err := instantiateSvc.Instantiate("firmware_instance", map[string]interface{}{
							"compVerTuple": compVerTuple,
							"name":         name,
							"version":      version,
							"updateable":   updateable,
							"installDate":  installDate,
							"id":           class,
						})

						// In the rare event that instantiate does not work, make sure not to add
						// the compVerTuple to the overall firmwareInventoryViews so that instantiate
						// can be retried.
						if err != nil {
							logger.Crit(compVerTuple + " swinv failed to instantiate: " + err.Error())
							continue
						}

						//fmt.Printf("add to list ---------> INSTANTIATED: %s\n", vw.GetURI())
						firmwareInventoryViews[compVerTuple] = vw

						// Mark URI as "in use" so that it is not deleted during clean up
						uris_in_use[vw.GetURI()] = true

					} else {
						vw := firmwareInventoryViews[compVerTuple]

						// Mark URI as "in use" so that it is not deleted during clean up
						uris_in_use[vw.GetURI()] = true

						firmMdl := vw.GetModel("default")

						tIntf, ok := firmMdl.GetPropertyOk("install_date")
						if !ok {
							continue
						}

						tStr, ok := tIntf.(string)
						if !ok {
							continue
						}

						t, _ := time.Parse(time.RFC3339, tStr)
						tBefore, _ := time.Parse(time.RFC3339, installDate)
						if !tBefore.Before(t) {
							firmMdl.UpdateProperty("install_date", installDate)
						}
					}

					// These values are for post processing on Instantiated object
					arr, ok := fqdd_mappings[compVerTuple]
					if !ok {
						arr = []string{}
					}

					if !in_array(fqdd, arr) {
						arr = append(arr, fqdd)
						sort.Strings(arr)
						fqdd_mappings[compVerTuple] = arr
					}

					uriarr, ok := uri_mappings[compVerTuple]
					if !ok {
						uriarr = []string{}
					}

					if !in_array(uri, uriarr) {
						uriarr = append(uriarr, uri)
						sort.Strings(uriarr)
						uri_mappings[compVerTuple] = uriarr
					}
				}
			}

			for compVerTuple, arr := range fqdd_mappings {
				vw := firmwareInventoryViews[compVerTuple]
				firmMdl := vw.GetModel("default")
				if firmMdl != nil {
					firmMdl.UpdateProperty("fqdd_list", arr)
				}
			}

			for compVerTuple, arr := range uri_mappings {
				vw := firmwareInventoryViews[compVerTuple]
				firmMdl := vw.GetModel("default")
				firmMdl.UpdateProperty("related_list", arr)
			}

			// Clean up any URIs which are no longer in use
			for uri, is_used := range uris_in_use {
				if !is_used {
					logger.Crit(uri + " swinv is no longer in use, removing")

					// Since the URI is no longer in use, delete it.
					if id, ok := d.GetAggregateIDOK(uri); ok {
						d.CommandHandler.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: id})
						delete(uris_in_use, uri)
						delete(firmwareInventoryViews, split_string_index(uri, "/", -1))
					} else {
						logger.Crit(uri + " swinv can't be deleted, could not find ID")
					}
				}
			}
		}
	}()
}

const (
	RequestFaultRemove = eh.EventType("Request:FaultRemove")
)

func init() {
	// implemented
	eh.RegisterCommand(func() eh.Command { return &FaultDELETE{} })
	eh.RegisterEventData(RequestFaultRemove, func() eh.EventData { return &RequestFaultRemoveData{} })
}

const (
	FaultDELETECommand = eh.CommandType("ECFault:DELETE")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&FaultDELETE{})

// HTTP DELETE Command
type FaultDELETE struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
}

type RequestFaultRemoveData struct {
	ID          eh.UUID // id of aggregate
	CmdID       string
	ResourceURI string
}

func (c *FaultDELETE) AggregateType() eh.AggregateType { return domain.AggregateType }
func (c *FaultDELETE) AggregateID() eh.UUID            { return c.ID }
func (c *FaultDELETE) CommandType() eh.CommandType     { return FaultDELETECommand }
func (c *FaultDELETE) SetAggID(id eh.UUID)             { c.ID = id }
func (c *FaultDELETE) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *FaultDELETE) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	// TODO: "Services may return a representation of the just deleted resource in the response body."
	// - can create a new CMD for GET with an identical CMD ID. Is that cheating?
	// TODO: return http 405 status for undeletable objects. right now we use privileges

	//data.Results, _ = ProcessDELETE(ctx, a.Properties, c.Body)

	faultID := ""
	// send event to trigger delete
	splitString := strings.Split(a.ResourceURI, "-")
	if len(splitString) == 2 {
		faultID = splitString[1]
	}
	fmt.Println(faultID)
	a.PublishEvent(eh.NewEvent(RequestFaultRemove, &RequestFaultRemoveData{
		ID:          c.ID,
		CmdID:       faultID,
		ResourceURI: a.ResourceURI,
	}, time.Now()))

	// send http response
	a.PublishEvent(eh.NewEvent(domain.HTTPCmdProcessed, &domain.HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{},
		StatusCode: 200,
		Headers:    map[string]string{},
	}, time.Now()))
	return nil
}
