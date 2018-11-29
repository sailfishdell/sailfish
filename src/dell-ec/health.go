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

	awesome_mapper2.AddFunction("generate_new_health", func(args ...interface{}) (interface{}, error) {
		healthEvent, ok := args[0].(*dm_event.HealthEventData)
		if !ok {
			logger.Crit("Mapper configuration error: health event data not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%#T", args[1]))
			return nil, errors.New("Mapper configuration error: health event data not passed")
		}
		aggregateUUID, ok := args[1].(eh.UUID)
		if !ok {
			logger.Crit("Mapper configuration error: aggregate UUID not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%#T", args[1]))
			return nil, errors.New("Mapper configuration error: aggregate UUID not passed")
		}

		s := strings.Split(healthEvent.FQDD, "#")
		subsys := s[len(s)-1]
		health := healthEvent.Health

		health_entry := map[string]interface{}{"Status": map[string]string{"HealthRollup": health}}
		subSystemHealthList[subsys] = health_entry

		if health == "Absent" || health == "" {
			if _, ok := subSystemHealthList[subsys]; ok { //property exists, delete
				delete(subSystemHealthList, subsys)
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
