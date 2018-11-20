package dell_ec

import (
	"context"
	"errors"
	"fmt"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

const MAX_LCLOGS = 10

func initLCL(logger log.Logger, ch eh.CommandHandler) {
	lclogs := []eh.UUID{}
	nextid := 0

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
		uri := fmt.Sprintf("%s/%d", logUri, nextid)
		nextid = nextid + 1

		lclogs = append(lclogs, uuid)

		go ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          uuid,
				ResourceURI: uri,
				Type:        "#LogServiceCollection.LogServiceCollection",
				Context:     "/redfish/v1/$metadata#LogServiceCollection.LogServiceCollection",
				Privileges: map[string]interface{}{
					"GET": []string{"ConfigureManager"},
				},
				Properties: map[string]interface{}{
					"Description": logEntry.Description,
					"Name":        logEntry.Name,
					"EntryType":   logEntry.EntryType,
					"Id":          logEntry.Id,
					"MessageArgs": logEntry.MessageArgs,
					"Message":     logEntry.Message,
					"MessageID":   logEntry.MessageID,
					"Category":    logEntry.Category,
					"Severity":    logEntry.Severity,
					"Action":      logEntry.Action,
				}})

		for len(lclogs) > MAX_LCLOGS {
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
