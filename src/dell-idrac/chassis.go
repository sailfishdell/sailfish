package dell_idrac

import (
	"errors"
	"sync"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
)

func AddChassisInstantiate(l log.Logger, instantiateSvc *testaggregate.Service) {
	// create hash and lock to keep track of the slots we instantiate
	chassisListMu := sync.Mutex{}
	chassisList := map[string]struct{}{}

	awesome_mapper2.AddFunction("addsystemchassis", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addslot(), but didnt get one")
		}
		l.Debug("called addslot from mapper", "FQDD", FQDD)

		chassisListMu.Lock()
		_, ok = chassisList[FQDD]
		if ok {
			l.Warn("Chassis already created, skip", "CHASSIS FQDD", FQDD)
			chassisListMu.Unlock()
			return false, nil
		}
		// track that this slot is instantiated
		chassisList[FQDD] = struct{}{}
		chassisListMu.Unlock()

		l.Info("About to instantiate", "FQDD", FQDD)

		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("system_chassis", map[string]interface{}{"FQDD": "System.Chassis.1"})

		return true, nil
	})
}
