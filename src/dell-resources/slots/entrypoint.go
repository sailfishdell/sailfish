package slots

import (
	"context"
	"errors"
	"fmt"
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

// StartService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
func CreateSlotCollection(ctx context.Context, baseView viewer, cfgMgr *viper.Viper, instantiateSvc *testaggregate.Service, modParams func(map[string]interface{}) map[string]interface{}) {
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
	awesome_mapper2.AddFunction("echo", func(args ...interface{}) (interface{}, error) {
		fmt.Println(args...)
		return true, nil
	})

	// create hash to keep track of the slots we instantiate
	slotsMu := sync.Mutex{}
	slots := map[string]struct{}{}
	var slotLogger log.Logger

	awesome_mapper2.AddFunction("addslot", func(args ...interface{}) (interface{}, error) {
		FQDD, ok := args[0].(string)
		if !ok {
			return nil, errors.New("Need a string fqdd for addslot(), but didnt get one")
		}
		slotLogger.Warn("called addslot from mapper", "FQDD", FQDD)

		s := strings.Split(FQDD, ".")
		group, index := s[0], s[1]

		slotsMu.Lock()
		_, ok = slots[FQDD]
		if ok {
			slotLogger.Warn("slot already created, skip", "baseSlotURI", myModParams()["collection_uri"], "SlotEntry.Id", FQDD)
			slotsMu.Unlock()
			return nil, nil
		}
		// track that this slot is instantiated
		slots[FQDD] = struct{}{}
		slotsMu.Unlock()

		slotLogger.Warn("About to instantiate", "FQDD", FQDD)
		// have to do this in a goroutine because awesome mapper is locked while it processes events
		go instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "slot",
			myModParams(map[string]interface{}{
				"FQDD":  FQDD,
				"Group": group, // for ar mapper
				"Index": index, // for ar mapper
			}),
		)
		slotLogger.Warn("Created Slot", "SlotEntry.Id", FQDD)

		return true, nil
	})

	slotLogger, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "slotcollection", myModParams())
	slotLogger.Warn("Created slot collection", "uri", baseView.GetURI()+"/Slots")
}
