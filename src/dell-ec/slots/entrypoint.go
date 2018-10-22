package slots

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
)

type viewer interface {
	GetURI() string
}

// CreateSlotCollection will instantiate the slot collection as well set up the function to add slots
func CreateSlotCollection(ctx context.Context, baseView viewer, cfgMgr *viper.Viper, instantiateSvc *testaggregate.Service, modParams func(map[string]interface{}) map[string]interface{}) {
	var slotLogger log.Logger
	myModParams := func(in ...map[string]interface{}) map[string]interface{} {
		mod := map[string]interface{}{}
		mod["collection_uri"] = baseView.GetURI() + "/Slots"
		for _, i := range in {
			for k, v := range i {
				mod[k] = v
			}
		}
		return modParams(mod)
	}

	// this is used in the closure below.
	slotsMu := sync.Mutex{}
	// create hash to keep track of the slots we instantiate
	slots := map[string]struct{}{}

	// in the future if we need to, we can create a map of key/values for which
	// URI to create the slot under, but for now there is only one place, so
	// make things as simple as possible.
	awesome_mapper2.AddFunction("addslot", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addslot(), but didnt get one")
		}
		slotLogger.Debug("called addslot from mapper", "FQDD", FQDD)

		s := strings.Split(FQDD, ".")
		group, index := s[0], s[1]

		slotsMu.Lock()
		_, ok = slots[FQDD]
		if ok {
			slotLogger.Warn("slot already created, skip", "baseSlotURI", myModParams()["collection_uri"], "SlotEntry.Id", FQDD)
			slotsMu.Unlock()
			return false, nil
		}
		// track that this slot is instantiated
		slots[FQDD] = struct{}{}
		slotsMu.Unlock()

		slotLogger.Info("About to instantiate", "FQDD", FQDD)
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "slot",
			myModParams(map[string]interface{}{
				"FQDD":  FQDD,
				"Group": group, // for ar mapper
				"Index": index, // for ar mapper
			}),
		)

		return true, nil
	})

	slotLogger, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "slotcollection", myModParams())
	slotLogger.Info("Created slot collection", "uri", baseView.GetURI()+"/Slots")
}

// CreateSlotConfigCollection will instantiate the slot collection as well set up the function to add slotconfigs
func CreateSlotConfigCollection(ctx context.Context, baseView viewer, cfgMgr *viper.Viper, instantiateSvc *testaggregate.Service, modParams func(map[string]interface{}) map[string]interface{}) {
	var slotLogger log.Logger
	myModParams := func(in ...map[string]interface{}) map[string]interface{} {
		mod := map[string]interface{}{}
		mod["collection_uri"] = baseView.GetURI() + "/SlotConfigs"
		for _, i := range in {
			for k, v := range i {
				mod[k] = v
			}
		}
		return modParams(mod)
	}

	// this is used in the closure below.
	slotConfigsMu := sync.Mutex{}
	// create hash to keep track of the slotConfigs we instantiate
	slotConfigs := map[string]struct{}{}

	// in the future if we need to, we can create a map of key/values for which
	// URI to create the slot under, but for now there is only one place, so
	// make things as simple as possible.
	awesome_mapper2.AddFunction("addslotconfig", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addslot(), but didnt get one")
		}
		slotLogger.Debug("called addslotconfig from mapper", "FQDD", FQDD)

		s := strings.Split(FQDD, ".")
		group, index := s[0], s[1]

		slotConfigsMu.Lock()
		_, ok = slotConfigs[FQDD]
		if ok {
			slotLogger.Warn("slotconfig already created, skip", "baseSlotURI", myModParams()["collection_uri"], "SlotEntry.Id", FQDD)
			slotConfigsMu.Unlock()
			return false, nil
		}
		// track that this slot is instantiated
		slotConfigs[FQDD] = struct{}{}
		slotConfigsMu.Unlock()

		slotLogger.Info("About to instantiate", "FQDD", FQDD)
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "slotconfig",
			myModParams(map[string]interface{}{
				"FQDD":  FQDD,
				"Group": group, // for ar mapper
				"Index": index, // for ar mapper
			}),
		)

		return true, nil
	})

	slotLogger, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "slotconfigcollection", myModParams())
	slotLogger.Info("Created slot config collection", "uri", baseView.GetURI()+"/SlotConfigs")
}
