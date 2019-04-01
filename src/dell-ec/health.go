package dell_ec

import (
	"context"
	"errors"
	"fmt"
	"strings"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func inithealth(ctx context.Context, logger log.Logger, ch eh.CommandHandler) {
	subSystemHealthList := map[string]interface{}{}

	awesome_mapper2.AddFunction("remove_health", func(args ...interface{}) (interface{}, error) {
		removedEvent, ok := args[0].(*dm_event.ComponentRemovedData)
		if !ok {
			logger.Crit("Mapper configuration error: component removed event data not passed", "args[0]", args[0], "TYPE", fmt.Sprintf("%T", args[0]))
			return nil, errors.New("Mapper configuration error: component removed event data not passed")
		}
		aggregateUUID, ok := args[1].(eh.UUID)
		if !ok {
			logger.Crit("Mapper configuration error: aggregate UUID not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("Mapper configuration error: aggregate UUID not passed")
		}
		subsys := removedEvent.Name
		for key, _ := range subSystemHealthList {
			if key == subsys {
				delete(subSystemHealthList, subsys)
				break
			}
		}
		//TODO: when iom is removed, is a new health event being sent out?
		// IF NOT: is odatalite updating health? if so this function needs
		//   to change the health of the component
		// IF SO: prevent 'generate_new_health' from remaking the subsystem

		ch.HandleCommand(ctx,
			&domain.RemoveRedfishResourceProperty{
				ID:       aggregateUUID,
				Property: subsys})

		return nil, nil
	})

	awesome_mapper2.AddFunction("generate_new_health", func(args ...interface{}) (interface{}, error) {
		healthEvent, ok := args[0].(*dm_event.HealthEventData)
		if !ok {
			logger.Crit("Mapper configuration error: health event data not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("Mapper configuration error: health event data not passed")
		}
		aggregateUUID, ok := args[1].(eh.UUID)
		if !ok {
			logger.Crit("Mapper configuration error: aggregate UUID not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("Mapper configuration error: aggregate UUID not passed")
		}

		s := strings.Split(healthEvent.FQDD, "#")
		subsys := s[len(s)-1]
		health := healthEvent.Health

		health_entry := map[string]interface{}{"Status": map[string]string{"HealthRollup": health}}
		if health == "" {
			health_entry = map[string]interface{}{"Status": map[string]interface{}{"HealthRollup": nil}}
		}
		subSystemHealthList[subsys] = health_entry

		// temporary workaround to filter out mchars health statuses
		// if additional subsystems end up getting incorrectly added, change pump to specify that only 5e, 1401, and 1303 events can be subsystems
		extra_subsys := []string{"Absent", "SledSystem", "IOM", "Group.1", "CMC.Integrated.1", "CMC.Integrated.2", "Root"}
		for _, extra := range extra_subsys {
			if extra == subsys {
				if _, ok := subSystemHealthList[subsys]; ok { //property exists, delete
					delete(subSystemHealthList, subsys)
				}
			}
		}

		// Ok, so this is a little wierd, sorry.
		// What we do here is directly update the aggregate because I cannot update the top level properties right now using the features built in.
		ch.HandleCommand(ctx,
			&domain.UpdateRedfishResourceProperties{
				ID:         aggregateUUID,
				Properties: subSystemHealthList})

		// to avoid extra memory usage, returning 'nil', but in the future should return subSystemHealthList when we can use it
		return nil, nil
	})

}
