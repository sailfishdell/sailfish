package dell_ec

import (
	"errors"
	"strings"
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

func AddECInstantiate(l log.Logger, instantiateSvc *testaggregate.Service) {

	MakeMaker(l, "ec_system_modular", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addec_system_modular(), but didnt get one")
		}
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("sled", map[string]interface{}{"FQDD": FQDD})

		return true, nil
	})

	MakeMaker(l, "iom", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addiom(), but didnt get one")
		}
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("iom", map[string]interface{}{"FQDD": FQDD})

		return true, nil
	})

	MakeMaker(l, "slot", func(args ...interface{}) (interface{}, error) {
		ParentFQDD, ok := args[1].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addidrac_storage_instance(), but didnt get one")
		}
		FQDD, ok := args[2].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addidrac_storage_instance(), but didnt get one")
		}

		s := strings.Split(FQDD, ".")
		group, index := s[0], s[1]

		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("slot",
			map[string]interface{}{
				"ParentFQDD": ParentFQDD,
				"FQDD":       FQDD,
				"Group":      group, // for ar mapper
				"Index":      index, // for ar mapper
			},
		)

		return true, nil
	})

	MakeMaker(l, "slotconfig", func(args ...interface{}) (interface{}, error) {
		ParentFQDD, ok := args[1].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addidrac_storage_instance(), but didnt get one")
		}
		FQDD, ok := args[2].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addidrac_storage_instance(), but didnt get one")
		}

		s := strings.Split(FQDD, ".")
		group, index := s[0], s[1]

		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("slotconfig",
			map[string]interface{}{
				"ParentFQDD": ParentFQDD,
				"FQDD":       FQDD,
				"Group":      group, // for ar mapper
				"Index":      index, // for ar mapper
			},
		)

		return true, nil
	})

}
