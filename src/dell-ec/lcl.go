package dell_ec

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func initLCL(logger log.Logger, instantiateSvc *testaggregate.Service, ch eh.CommandHandler, d *domain.DomainObjects) {
	MAX_LOGS := 51
	lclogs := []eh.UUID{}

	awesome_mapper2.AddFunction("addlclog", func(args ...interface{}) (interface{}, error) {
		logUri, ok := args[0].(string)
		if !ok {
			logger.Crit("Mapper configuration error: uri not passed as string", "args[0]", args[0])
			return nil, errors.New("Mapper configuration error: uri not passed as string")
		}

		logEntry, ok := args[1].(*LogEventData)
		if !ok {
			logger.Crit("Mapper configuration error: log event data not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%#T", args[1]))
			return nil, errors.New("Mapper configuration error: log event data not passed")
		}

		uuid := eh.NewUUID()
		uri := fmt.Sprintf("%s/%d", logUri, logEntry.Id)

		aggID, ok := d.GetAggregateIDOK(uri)
		if ok {
			logger.Crit("Mapper configuration error: URI already exists", "aggID", aggID, "uri", uri)
			return nil, errors.New("lclog: URI already exists %s")
		}

		timeF, err := strconv.ParseFloat(logEntry.Created, 64)
		if err != nil {
			logger.Crit("Mapper configuration error: Time information can not be parsed", "time", logEntry.Created, "err", err)
			return nil, errors.New("Mapper configuration error: Time information can not be parsed")
		}
		createdTime := time.Unix(int64(timeF), 0)

		severity := logEntry.Severity
		if logEntry.Severity == "Informational" {
			severity = "OK"
		}

		lclogs = append(lclogs, uuid)

		go ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          uuid,
				ResourceURI: uri,
				Type:        "#LogEntry.v1_0_2.LogEntry",
				Context:     "/redfish/v1/$metadata#LogEntry.LogEntry",
				Privileges: map[string]interface{}{
					"GET": []string{"ConfigureManager"},
				},
				Properties: map[string]interface{}{
					"Created":     createdTime,
					"Description": logEntry.Name,
					"Name":        logEntry.Name,
					"EntryType":   logEntry.EntryType,
					"Id":          logEntry.Id,
					"Links": map[string]interface{}{
						"OriginOfCondition": map[string]interface{}{
							"@odata.id": "/redfish/v1/Chassis/System.Chassis.1",
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
						}},
					"OemRecordFormat": "Dell",
					"Severity":        severity,
					"Action":          logEntry.Action,
				}})
		// need to be updated to filter the first 50...
		for len(lclogs) > MAX_LOGS {
			logger.Debug("too many logs, trimming", "len", len(lclogs))
			toDelete := lclogs[0]
			lclogs = lclogs[1:]
			go ch.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: toDelete})
		}

		return true, nil
	})

	awesome_mapper2.AddFunction("addfaultentry", func(args ...interface{}) (interface{}, error) {
		logUri, ok := args[0].(string)
		if !ok {
			logger.Crit("Mapper configuration error: uri not passed as string", "args[0]", args[0])
			return nil, errors.New("Mapper configuration error: uri not passed as string")
		}
		faultEntry, ok := args[1].(*FaultEntryAddData)
		if !ok {
			logger.Crit("Mapper configuration error: log event data not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%#T", args[1]))
			return nil, errors.New("Mapper configuration error: log event data not passed")
		}

		uuid := eh.NewUUID()
		uri := fmt.Sprintf("%s/%d", logUri, faultEntry.Id)

		go ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          uuid,
				ResourceURI: uri,
				Type:        "#LogEntryCollection.LogEntryCollection",
				Context:     "/redfish/v1/$metadata#LogEntryCollection.LogEntryCollection",
				Privileges: map[string]interface{}{
					"GET": []string{"ConfigureManager"},
				},
				Properties: map[string]interface{}{
					"Description": faultEntry.Description,
					"Name":        faultEntry.Name,
					"EntryType":   faultEntry.EntryType,
					"Id":          faultEntry.Id,
					"MessageArgs": faultEntry.MessageArgs,
					"Message":     faultEntry.Message,
					"MessageID":   faultEntry.MessageID,
					"Category":    faultEntry.Category,
					"Severity":    faultEntry.Severity,
					"Action":      faultEntry.Action,
				}})

		return true, nil
	})

	awesome_mapper2.AddFunction("has_swinv_model", func(args ...interface{}) (interface{}, error) {
		//fmt.Printf("Check to see if the new resource has an 'swinv' model\n")

		resourceURI, ok := args[0].(string)
		if !ok || resourceURI == "" {
			//fmt.Printf("no resource uri passed or not string\n")
			return false, nil
		}

		v, err := domain.InstantiatePlugin(domain.PluginType(resourceURI))
		if err != nil || v == nil {
			//fmt.Printf("couldn't instantiate view for URI (%s): %s\n", resourceURI, err)
			return false, nil
		}

		vw, ok := v.(*view.View)
		if !ok {
			//fmt.Printf("instantiated non-view\n")
			return false, nil
		}

		mdl := vw.GetModel("swinv")
		if mdl == nil {
			//fmt.Printf("NO SWINV MODEL (not an error)\n")
			return false, nil
		}

		return true, nil
	})

	var syncModels func(m *model.Model, updates []model.Update)
	type newfirm struct {
		uri string
		mdl *model.Model
	}
	newchan := make(chan newfirm, 30)
	trigger := make(chan struct{})
	firmwareInventoryViews := map[string]*view.View{}

	awesome_mapper2.AddFunction("add_swinv", func(args ...interface{}) (interface{}, error) {
		resourceURI, ok := args[0].(string)
		if !ok || resourceURI == "" {
			fmt.Printf("OUT 1\n")
			return false, nil
		}

		v, err := domain.InstantiatePlugin(domain.PluginType(resourceURI))
		if err != nil || v == nil {
			fmt.Printf("OUT 2\n")
			return false, nil
		}

		vw, ok := v.(*view.View)
		if !ok {
			fmt.Printf("OUT 3\n")
			return false, nil
		}

		mdl := vw.GetModel("swinv")
		if mdl == nil {
			fmt.Printf("OUT 4\n")
			return false, nil
		}

		mdl.AddObserver("swinv", syncModels)

		newchan <- newfirm{resourceURI, mdl}

		return true, nil
	})

	syncModels = func(m *model.Model, updates []model.Update) {
		select {
		case trigger <- struct{}{}:
		default:
		}
	}

	go func() {
		swinvList := map[string]*model.Model{}
		for {

			// Wait for this thread to be kicked
			// either a model gets updated (trigger)
			// or a new model is added (newchan)
			select {
			case <-trigger:
			case n := <-newchan:
				swinvList[n.uri] = n.mdl
				fmt.Printf("NEW model from URI: %s\n", n.uri)
				continue
			}

			// scan through each model and build our new inventory uris
			for uri, mdl := range swinvList {
				fqddRaw, ok := mdl.GetPropertyOk("fw_fqdd")
				if !ok || fqddRaw == nil {
					logger.Debug("DID NOT GET fqdd raw")
					continue
				}

				fqdd, ok := fqddRaw.(string)
				if !ok || fqdd == "" {
					logger.Debug("DID NOT GET fqdd stringg")
					continue
				}

				classRaw, ok := mdl.GetPropertyOk("fw_device_class")
				if !ok || classRaw == nil {
					logger.Debug("DID NOT GET device_class raw")
					continue
				}

				class, ok := classRaw.(string)
				if !ok || class == "" {
					logger.Debug("DID NOT GET class string")
					class = "unknown"
				}

				versionRaw, ok := mdl.GetPropertyOk("fw_version")
				if !ok || versionRaw == nil {
					logger.Debug("DID NOT GET version raw")
					continue
				}

				version, ok := versionRaw.(string)
				if !ok || version == "" {
					logger.Debug("DID NOT GET version string")
					version = "unknown"
				}

				compVerTuple := class + "-" + version
				fw_fqdd_list := []string{fqdd}
				fw_related_list := []map[string]interface{}{}

				_ = uri
				_ = fw_fqdd_list
				_ = fw_related_list

				if _, ok := firmwareInventoryViews[compVerTuple]; !ok {
					_, vw, _ := instantiateSvc.InstantiateNoWait("firmware_instance", map[string]interface{}{
						"compVerTuple": compVerTuple,
						"name":         "TEST",
						"version":      version,
						"updateable":   false,
						"id":           class,
					})
					fmt.Printf("add to list ------------------------------ INSTANTIATED: %s\n", vw.GetURI())
					firmwareInventoryViews[compVerTuple] = vw
				}
			}
		}
	}()
}
