package dell_idrac

import (
	"errors"
	"sync"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
)

func MakeMaker(l log.Logger, name string, fn func(args ...interface{}) (interface{}, error)) {
	// create hash and lock to keep track of the things we instantiate
	setMu := sync.Mutex{}
	set := map[string]struct{}{}

	awesome_mapper2.AddFunction("add"+name, func(args ...interface{}) (interface{}, error) {
		uniqueName, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addslot(), but didnt get one")
		}
		l.Debug("called add"+name+" from mapper", "uniqueName", uniqueName)

		setMu.Lock()
		_, ok = set[uniqueName]
		if ok {
			l.Warn("Already created unique name in this set", "name", name, "uniqueName", uniqueName)
			setMu.Unlock()
			return false, nil
		}
		// track that this slot is instantiated
		set[uniqueName] = struct{}{}
		setMu.Unlock()

		l.Info("About to instantiate", "uniqueName", uniqueName)
		return fn(args...)
	})
}

func AddChassisInstantiate(l log.Logger, instantiateSvc *testaggregate.Service) {
	MakeMaker(l, "system_embedded", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addsystem_embedded(), but didnt get one")
		}
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("idrac_system_embedded", map[string]interface{}{"FQDD": FQDD})

		return true, nil
	})

	MakeMaker(l, "system_chassis", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addsystem_chassis(), but didnt get one")
		}
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("system_chassis", map[string]interface{}{"FQDD": FQDD})

		return true, nil
	})

	MakeMaker(l, "idrac_storage_instance", func(args ...interface{}) (interface{}, error) {
		ParentFQDD, ok := args[1].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addidrac_storage_instance(), but didnt get one")
		}
		FQDD, ok := args[2].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addidrac_storage_instance(), but didnt get one")
		}
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("idrac_storage_instance", map[string]interface{}{"ParentFQDD": ParentFQDD, "FQDD": FQDD})

		return true, nil
	})

	MakeMaker(l, "idrac_storage_drive", func(args ...interface{}) (interface{}, error) {
		ParentFQDD, ok := args[1].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addidrac_storage_drive(), but didnt get one")
		}
		FQDD, ok := args[2].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addidrac_storage_drive(), but didnt get one")
		}
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		l.Warn("instantiate idrac_storage_drive", "ParentFQDD", ParentFQDD, "FQDD", FQDD)
		go instantiateSvc.Instantiate("idrac_storage_drive", map[string]interface{}{"ParentFQDD": ParentFQDD, "FQDD": FQDD})

		return true, nil
	})


	MakeMaker(l, "storage_enclosure", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addstorage_enclosure(), but didnt get one")
		}
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("storage_enclosure", map[string]interface{}{"URI_FQDD": FQDD, "EVENT_FQDD": "308|C|" + FQDD})

		return true, nil
	})
}
