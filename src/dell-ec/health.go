package dell_ec

import (
	"errors"
	"fmt"
	"strings"

	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
)

func inithealth(logger log.Logger) {
	subsystem_healths := map[string]interface{}{}

	awesome_mapper2.AddFunction("generate_new_health", func(args ...interface{}) (interface{}, error) {
		healthEvent, ok := args[0].(*dm_event.HealthEventData)
		if !ok {
			logger.Crit("Mapper configuration error: health event data not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%#T", args[1]))
			return nil, errors.New("Mapper configuration error: health event data not passed")
		}

		s := strings.Split(healthEvent.FQDD, "#")
		subsys := s[len(s)-1]
		health := healthEvent.Health

		health_entry := map[string]interface{}{"Status": map[string]string{"HealthRollup": health}}
		subsystem_healths[subsys] = health_entry

		if health == "Absent" || health == "" {
			if _, ok := subsystem_healths[subsys]; ok { //property exists, delete
				delete(subsystem_healths, subsys)
			}
		}

		return subsystem_healths, nil
	})

}
