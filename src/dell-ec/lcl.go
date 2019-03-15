package dell_ec

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func in_array(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func link_mapper(fqdd string) string {
	ret_string := "/redfish/v1/Chassis/System.Chassis.1"
	if strings.HasPrefix(fqdd, "Fan") {
		ret_string += "/Sensors/Fans/" + fqdd
	} else if strings.HasPrefix(fqdd, "PSU") {
		ret_string += "/Sensors/PowerSupplies/" + fqdd
	}
	return ret_string
}

func initLCL(logger log.Logger, instantiateSvc *testaggregate.Service, ch eh.CommandHandler, d *domain.DomainObjects) {
	MAX_LOGS := 3000

	awesome_mapper2.AddFunction("addlclog", func(args ...interface{}) (interface{}, error) {
		logUri, ok := args[0].(string)
		if !ok {
			logger.Crit("Mapper configuration error: uri not passed as string", "args[0]", args[0])
			return nil, errors.New("Mapper configuration error: uri not passed as string")
		}

		logEntry, ok := args[1].(*LogEventData)
		if !ok {
			logger.Crit("Mapper configuration error: log event data not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("Mapper configuration error: log event data not passed")
		}

		uuid := eh.NewUUID()
		uri := fmt.Sprintf("%s/%d", logUri, logEntry.Id)

		timeF, err := strconv.ParseFloat(logEntry.Created, 64)
		if err != nil {
			logger.Debug("LCLOG: Time information can not be parsed", "time", logEntry.Created, "err", err, "set time to", 0)
			timeF = 0
		}
		createdTime := time.Unix(int64(timeF), 0)
    cTime := createdTime.Format("2006-01-02T15:04:05-07:00")

		severity := logEntry.Severity
		if logEntry.Severity == "Informational" {
			severity = "OK"
		}

		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          uuid,
				ResourceURI: uri,
				Type:        "#LogEntry.v1_0_2.LogEntry",
				Context:     "/redfish/v1/$metadata#LogEntry.LogEntry",
				Privileges: map[string]interface{}{
					"GET": []string{"Login"},
				},
				Properties: map[string]interface{}{
          "Created": cTime,
					"Description": logEntry.Name,
					"Name":        logEntry.Name,
					"EntryType":   logEntry.EntryType,
					"Id":          logEntry.Id,
					"Links": map[string]interface{}{
						"OriginOfCondition": map[string]interface{}{
							"@odata.id": link_mapper(logEntry.FQDD),
						},
					},
					"MessageArgs@odata.count": len(logEntry.MessageArgs),
					"MessageArgs":             logEntry.MessageArgs,
					"Message":                 logEntry.Message,
					"MessageId":               logEntry.MessageID,
					"Oem": map[string]interface{}{
						"Dell": map[string]interface{}{
							"@odata.type": "#DellLogEntry.v1_0_0.LogEntrySummary",
							"Category":    logEntry.Category,
							"FQDD":        logEntry.FQDD,
						}},
					"OemRecordFormat": "Dell",
					"Severity":        severity,
					"Action":          logEntry.Action,
				}})

		uriList := d.FindMatchingURIs(func(uri string) bool { return path.Dir(uri) == logUri })

		sort.Slice(uriList, func(i, j int) bool {
			idx_i, _ := strconv.Atoi(path.Base(uriList[i]))
			idx_j, _ := strconv.Atoi(path.Base(uriList[j]))
			return idx_i > idx_j
		})

		if len(uriList) > MAX_LOGS {
			for _, uri := range uriList[MAX_LOGS:] {
				logger.Debug("too many logs, trimming", "len", len(uriList))
				id, ok := d.GetAggregateIDOK(uri)
				if ok {
					ch.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: id})
				}
			}
		}

		return true, nil
	})

	awesome_mapper2.AddFunction("clearuris", func(args ...interface{}) (interface{}, error) {
		logUri, ok := args[0].(string)
		if !ok {
			logger.Crit("Mapper configuration error: uri not passed as string", "args[0]", args[0])
			return nil, errors.New("Mapper configuration error: uri not passed as string")
		}

		logger.Debug("Clearing all uris within base_uri", "base_uri", logUri)

		uriList := d.FindMatchingURIs(func(uri string) bool { return path.Dir(uri) == logUri })
		var idList []eh.UUID
		for _, uri := range uriList {
			id, ok := d.GetAggregateIDOK(uri)
			if ok {
				idList = append(idList, id)
			}
		}
		go func(toDel []eh.UUID) {
			for idx, id := range idList {
				ch.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: id})
				if idx%10 == 0 {
					//Ugh... but slow it down so we don't flood the queue and deadlock
					time.Sleep(time.Second / 10)
				}
			}
		}(idList)

		return nil, nil
	})
	awesome_mapper2.AddFunction("removefaultentry", func(args ...interface{}) (interface{}, error) {
		logUri, ok := args[0].(string)
		if !ok {
			logger.Crit("Mapper configuration error: uri not passed as string", "args[0]", args[0])
			return nil, errors.New("Mapper configuration error: uri not passed as string")
		}
		faultEntry, ok := args[1].(*FaultEntryRmData)
		if !ok {
			logger.Crit("Mapper configuration error: log event data not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("Mapper configuration error: log event data not passed")
		}

		uri := fmt.Sprintf("%s/%s", logUri, faultEntry.Name)
		//fmt.Printf("%s/%s", logUri, faultEntry.Name)

		id, ok := d.GetAggregateIDOK(uri)
		if ok {
			ch.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: id})
		}
		return true, nil
	})

	awesome_mapper2.AddFunction("addfaultentry", func(args ...interface{}) (interface{}, error) {
		logUri, ok := args[0].(string)
		//fmt.Printf("%s", logUri)
		if !ok {
			logger.Crit("Mapper configuration error: uri not passed as string", "args[0]", args[0])
			return nil, errors.New("Mapper configuration error: uri not passed as string")
		}

		faultEntry, ok := args[1].(*FaultEntryAddData)
		if !ok {
			logger.Crit("Mapper configuration error: log event data not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("Mapper configuration error: log event data not passed")
		}

		timeF, err := strconv.ParseFloat(faultEntry.Created, 64)
		if err != nil {
			logger.Debug("Mapper configuration error: Time information can not be parsed", "time", faultEntry.Created, "err", err, "set time to", 0)
			timeF = 0
		}
		createdTime := time.Unix(int64(timeF), 0)
    cTime := createdTime.Format("2006-01-02T15:04:05-07:00")

		uuid := eh.NewUUID()
		uri := fmt.Sprintf("%s/%s", logUri, faultEntry.Name)
		//fmt.Printf("%s/%s", logUri, faultEntry.Name)

		// when mchars is restarted, it clears faults and expects old faults to be recreated.
		// skip re-creating old faults if this happens.
		aggID, ok := d.GetAggregateIDOK(uri)
		if ok {
			logger.Info("URI already exists, skipping add log", "aggID", aggID, "uri", uri)
			// not returning error because that will unnecessarily freak out govaluate when there really isn't an error we care about at that level
			return nil, nil
		}

		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          uuid,
				ResourceURI: uri,
				Type:        "#LogEntryCollection.LogEntryCollection",
				Context:     "/redfish/v1/$metadata#LogEntryCollection.LogEntryCollection",
				Headers: map[string]string{
					"Location": uri,
				},
				Privileges: map[string]interface{}{
					"GET": []string{"Login"},
				},
				Properties: map[string]interface{}{
          "Created": cTime,
					"Description":             "FaultList Entry " + faultEntry.FQDD,
					"Name":                    "FaultList Entry " + faultEntry.FQDD,
					"EntryType":               faultEntry.EntryType,
					"Id":                      faultEntry.Name,
					"MessageArgs":             faultEntry.MessageArgs,
					"MessageArgs@odata.count": len(faultEntry.MessageArgs),
					"Message":                 faultEntry.Message,
					"MessageId":               faultEntry.MessageID,
					"Category":                faultEntry.Category,
					"Oem": map[string]interface{}{
						"Dell": map[string]interface{}{
							"@odata.type": "#DellLogEntry.v1_0_0.LogEntrySummary",
							"FQDD":        faultEntry.FQDD,
							"SubSystem":   faultEntry.SubSystem,
						}},
					"OemRecordFormat": "Dell",
					"Severity":        faultEntry.Severity,
					"Action":          faultEntry.Action,
					"Links":           map[string]interface{}{},
				}})

		return true, nil
	})

	awesome_mapper2.AddFunction("firealert", func(args ...interface{}) (interface{}, error) {
		logEntry, ok := args[0].(*LogEventData)
		if !ok {
			logger.Crit("Mapper configuration error: log event data not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("Mapper configuration error: log event data not passed")
		}

		timeF, err := strconv.ParseFloat(logEntry.Created, 64)
		if err != nil {
			logger.Debug("Mapper configuration error: Time information can not be parsed", "time", logEntry.Created, "err", err, "set time to", 0)
			timeF = 0
		}
		createdTime := time.Unix(int64(timeF), 0)
    cTime := createdTime.Format("2006-01-02T15:04:05-07:00")

		//Create Alert type event:

		d.EventBus.PublishEvent(context.Background(),
			eh.NewEvent(eventservice.RedfishEvent, &eventservice.RedfishEventData{
				EventType:      "Alert",
				EventId:        logEntry.EventId,
        EventTimestamp: cTime,
				Severity:       logEntry.Severity,
				Message:        logEntry.Message,
				MessageId:      logEntry.MessageID,
				MessageArgs:    logEntry.MessageArgs,
				//TODO MSM BUG: OriginOfCondition for events has to be a string or will be rejected
				OriginOfCondition: logEntry.FQDD,
			}, time.Now()))

		return true, nil
	})

	awesome_mapper2.AddFunction("has_swinv_model", func(args ...interface{}) (interface{}, error) {
		//fmt.Printf("Check to see if the new resource has an 'swinv' model\n")

		resourceURI, ok := args[0].(string)
		if !ok || resourceURI == "" {
			//fmt.Printf("has_swinv: no resource uri passed or not string\n")
			return false, nil
		}

		//fmt.Printf("has_swinv URI (%s)\n", resourceURI)

		v, err := domain.InstantiatePlugin(domain.PluginType(resourceURI))
		if err != nil || v == nil {
			//fmt.Printf("has_swinv couldn't instantiate view for URI (%s): %s\n", resourceURI, err)
			return false, nil
		}

		vw, ok := v.(*view.View)
		if !ok {
			//fmt.Printf("has_swinv instantiated non-view\n")
			return false, nil
		}

		mdl := vw.GetModel("swinv")
		if mdl == nil {
			//fmt.Printf("has_swinv NO SWINV MODEL (not an error)\n")
			return false, nil
		}

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

	awesome_mapper2.AddFunction("add_swinv", func(args ...interface{}) (interface{}, error) {
		resourceURI, ok := args[0].(string)
		if !ok || resourceURI == "" {
			return false, nil
		}

		v, err := domain.InstantiatePlugin(domain.PluginType(resourceURI))
		if err != nil || v == nil {
			return false, nil
		}

		vw, ok := v.(*view.View)
		if !ok {
			return false, nil
		}

		mdls := vw.GetModels("swinv")
		if mdls == nil {
			return false, nil
		}

		for _, mdl := range mdls {
			//fmt.Printf("passing swinv to channel with uri %s\n", resourceURI)
			mdl.AddObserver("swinv", syncModels)
		}

		newchan <- newfirm{resourceURI, mdls}

		//fmt.Printf("URI is in %s\n", resourceURI)
		return true, nil
	})

	syncModels = func(m *model.Model, updates []model.Update) {
		select {
		case trigger <- struct{}{}:
		default:
		}
	}

	go func() {
		swinvList := map[string][]*model.Model{}
		for {

			// Wait for this thread to be kicked
			// either a model gets updated (trigger)
			// or a new model is added (newchan)
			select {
			case <-trigger:
			case n := <-newchan:
				swinvList[n.uri] = n.mdls
				//fmt.Printf("NEW model from URI: %s\n", n.uri)
				continue
			}

			fqdd_mappings := map[string][]string{}
			uri_mappings := map[string][]string{}

			// scan through each model and build our new inventory uris
			// need to iterate through models.  With the same work flow below.. When iteration is complete... need to add uris and fqdds to model.
			for uri, mdls := range swinvList {
				for _, mdl := range mdls {
					//fmt.Printf("swinvList uri is %s\n", uri)
					fqddRaw, ok := mdl.GetPropertyOk("fw_fqdd")
					if !ok || fqddRaw == nil {
						logger.Debug("swinv DID NOT GET fqdd raw with " + uri)
						continue
					}

					fqdd, ok := fqddRaw.(string)
					if !ok || fqdd == "" {
						logger.Debug("swinv DID NOT GET fqdd string with " + uri)
						continue
					}

					classRaw, ok := mdl.GetPropertyOk("fw_device_class")
					if !ok || classRaw == nil {
						logger.Debug("swinv DID NOT GET device_class raw with " + uri)
						continue
					}

					class, ok := classRaw.(string)
					if !ok || class == "" {
						logger.Debug("swinv DID NOT GET class string with " + uri)
						class = "unknown"
					}

					versionRaw, ok := mdl.GetPropertyOk("fw_version")
					if !ok || versionRaw == nil {
						logger.Debug("swinv DID NOT GET version raw with " + uri)
						continue
					}

					version, ok := versionRaw.(string)
					if !ok || version == "" {
						logger.Debug("swinv DID NOT GET version string with " + uri)
						version = "unknown"
					}

					compVerTuple := "Installed-" + class + "-" + version

					updateableRaw, ok := mdl.GetPropertyOk("fw_updateable")
					if !ok {
						logger.Debug("swinv DID NOT GET updateable string with " + uri)
						continue
					}

					updateable, ok := updateableRaw.(string)
					if !ok || updateable == "" || updateable == "Yes" {
						updateable = "True"
					} else {
						updateable = "False"
					}

					installDateRaw, ok := mdl.GetPropertyOk("fw_install_date")
					if !ok {
						logger.Debug("swinv DID NOT GET install date string with " + uri)
						continue
					}

					installDate, ok := installDateRaw.(string)
					if !ok || installDate == "" {
						logger.Debug("swinv DID NOT GET class string with " + uri)
						installDate = "1970-01-01T00:00:00Z"
					}

					nameRaw, ok := mdl.GetPropertyOk("fw_name")
					if !ok {
						logger.Debug("swinv DID NOT GET name string with " + uri)
						continue
					}

					name, ok := nameRaw.(string)
					if !ok || name == "" {
						logger.Debug("swinv DID NOT GET name string with " + uri)
						name = ""
					}

					if _, ok := firmwareInventoryViews[compVerTuple]; !ok {
						_, vw, _ := instantiateSvc.Instantiate("firmware_instance", map[string]interface{}{
							"compVerTuple": compVerTuple,
							"name":         name,
							"version":      version,
							"updateable":   updateable,
							"installDate":  installDate,
							"id":           class,
						})
						//fmt.Printf("add to list ---------> INSTANTIATED: %s\n", vw.GetURI())
						firmwareInventoryViews[compVerTuple] = vw
					}

					// These values are for post processing on Instantiated object
					arr, ok := fqdd_mappings[compVerTuple]
					if !ok {
						arr = []string{}
					}
					arr = append(arr, fqdd)
					fqdd_mappings[compVerTuple] = arr

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
				mdl := vw.GetModel("default")
				if mdl != nil {
					mdl.UpdateProperty("fqdd_list", arr)
				}
			}

			for compVerTuple, arr := range uri_mappings {
				vw := firmwareInventoryViews[compVerTuple]
				mdl := vw.GetModel("default")
				mdl.UpdateProperty("related_list", arr)
			}
		}
	}()
}
