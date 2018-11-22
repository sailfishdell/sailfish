package dell_ec

import (
	"errors"
	"strings"
	"sync"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
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

	MakeMaker(l, "manager_cmc_integrated", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addec_system_modular(), but didnt get one")
		}
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("manager_cmc_integrated", map[string]interface{}{
			"FQDD": FQDD,
			"exportSystemConfiguration":        view.Action(exportSystemConfiguration),
			"importSystemConfiguration":        view.Action(importSystemConfiguration),
			"importSystemConfigurationPreview": view.Action(importSystemConfigurationPreview),
		})

		return true, nil
	})

	MakeMaker(l, "system_chassis", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addec_system_modular(), but didnt get one")
		}
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("system_chassis", map[string]interface{}{
			"FQDD":                   FQDD,
			"msmConfigBackup":        view.Action(msmConfigBackup),
			"chassisMSMConfigBackup": view.Action(chassisMSMConfigBackup),
		})

		return true, nil
	})

	MakeMaker(l, "chassis_cmc_integrated", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addec_system_modular(), but didnt get one")
		}
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("chassis_cmc_integrated", map[string]interface{}{"FQDD": FQDD})

		return true, nil
	})

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

	MakeMaker(l, "ecfan", func(args ...interface{}) (interface{}, error) {
		ParentFQDD, ok := args[1].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addecfan(), but didnt get one")
		}
		FQDD, ok := args[2].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addecfan(), but didnt get one")
		}

		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("fan",
			map[string]interface{}{
				"ChassisFQDD": ParentFQDD,
				"FQDD":        FQDD,
			},
		)

		return true, nil
	})

	MakeMaker(l, "ecpsu", func(args ...interface{}) (interface{}, error) {
		ParentFQDD, ok := args[1].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addecpsu(), but didnt get one")
		}
		FQDD, ok := args[2].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addecpsu(), but didnt get one")
		}

		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("psu_slot",
			map[string]interface{}{
				"DM_FQDD":     "System.Chassis.1#" + strings.Replace(FQDD, "PSU.Slot", "PowerSupply", 1),
				"ChassisFQDD": ParentFQDD,
				"FQDD":        FQDD,
			},
		)

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
		if len(s) < 2 {
			return nil, errors.New("Invalid FQDD")
		}
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
		if len(s) < 2 {
			return nil, errors.New("Invalid FQDD")
		}
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

	MakeMaker(l, "certificate", func(args ...interface{}) (interface{}, error) {
		ParentFQDD, ok := args[1].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addcertificate(), but didnt get one")
		}
		FQDD, ok := args[2].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addcertificate(), but didnt get one")
		}

		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.Instantiate("certificate",
			map[string]interface{}{
				"ParentFQDD": ParentFQDD,
				"FQDD":       FQDD,
			},
		)

		return true, nil
	})

}
