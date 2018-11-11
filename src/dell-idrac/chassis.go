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
		go instantiateSvc.Instantiate("system_chassis", map[string]interface{}{"FQDD": FQDD})

		return true, nil
	})


	// create hash and lock to keep track of the slots we instantiate
	storageEnclosureListMu := sync.Mutex{}
	storageEnclosureList := map[string]struct{}{}

	awesome_mapper2.AddFunction("addstorageenclosure", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addslot(), but didnt get one")
		}
		l.Debug("called addslot from mapper", "FQDD", FQDD)

		storageEnclosureListMu.Lock()
		_, ok = storageEnclosureList[FQDD]
		if ok {
			l.Warn("Storage Enclosure already created, skip", "CHASSIS FQDD", FQDD)
			storageEnclosureListMu.Unlock()
			return false, nil
		}
		// track that this slot is instantiated
		storageEnclosureList[FQDD] = struct{}{}
		storageEnclosureListMu.Unlock()

		l.Info("About to instantiate", "FQDD", FQDD)

		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("storage_enclosure", map[string]interface{}{"URI_FQDD": FQDD, "EVENT_FQDD": "308|C|" + FQDD})

		return true, nil
	})


}
