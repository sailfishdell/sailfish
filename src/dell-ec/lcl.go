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
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func initLCL(logger log.Logger, ch eh.CommandHandler, d *domain.DomainObjects) {
	MAX_LOGS :=51
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
			logger.Crit("Mapper configuration error: URI already exists", "aggID",aggID, "uri",uri)
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

}
