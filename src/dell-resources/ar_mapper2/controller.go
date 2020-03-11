package ar_mapper2

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/model"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	a "github.com/superchalupa/sailfish/src/dell-resources/attributedef"
)

// what we read from redfish.yaml
type mapping struct {
	Property string
	FQDD     string
	Group    string
	Index    string
	Name     string
}

// individual mapping: only for bookkeeping
type ModelMappings struct {
	mappings       []mapping
	cfgsect        string
	model          *model.Model
	requestedFQDD  string
	requestedGroup string
	requestedIndex string
	loaded         bool
}

type ARService struct {
	eb     eh.EventBus
	cfg    *viper.Viper
	cfgMu  *sync.RWMutex
	logger log.Logger

	mappingsMu    sync.RWMutex
	modelmappings map[string]ModelMappings

	hashMu    sync.RWMutex
	hash      map[string][]update
	hashDirty bool

	ew *eventwaiter.EventWaiter
}

// post-processed and optimized update
type update struct {
	model    *model.Model
	property string
}

type BusObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
	GetCommandHandler() eh.CommandHandler
}

func StartService(ctx context.Context, logger log.Logger, cfg *viper.Viper, cfgMu *sync.RWMutex, d BusObjs) (*ARService, error) {
	logger = log.With(logger, "module", "ar2")

	arservice := &ARService{
		eb:            d.GetBus(),
		cfg:           cfg,
		cfgMu:         cfgMu,
		logger:        logger,
		modelmappings: make(map[string]ModelMappings),
		hash:          make(map[string][]update),
		hashDirty:     true,
		ew:            d.GetWaiter(),
	}

	listener := eventwaiter.NewListener(ctx, logger, arservice.ew, func(ev eh.Event) bool {
		return ev.EventType() == a.AttributeUpdated
	})
	// never calling listener.Close() because we can't shut this down

	go listener.ProcessEvents(ctx, func(event eh.Event) {
		data, ok := event.Data().(*a.AttributeUpdatedData)
		if !ok {
			return
		}

		logger.Debug("processing event", "event", event)
		key := fmt.Sprintf("%s:%s:%s:%s", data.FQDD, data.Group, data.Index, data.Name)

		arservice.hashMu.RLock()
		if arservice.hashDirty {

			arservice.hashMu.RUnlock()

			// drop locks before calling
			arservice.optimizeHash()

			arservice.hashMu.RLock()
		}

		if arr, ok := arservice.hash[key]; ok {
			logger.Debug("matched quick hash", "key", key)
			for _, u := range arr {
				if data.Error == "" {
					logger.Debug("updating property", "property", u.property, "value", data.Value)
					u.model.UpdateProperty(u.property, data.Value)
				} else {
					logger.Debug("event has error, not updating property", "property", u.property, "error", data.Error)
				}
			}
		}
		arservice.hashMu.RUnlock()
	})

	return arservice, nil
}

// used by individual mappings to keep track of the mapping name for PUT/PATCH/POST, etc.
type breadcrumb struct {
	ars         *ARService
	mappingName string
}

func (ars *ARService) NewMapping(logger log.Logger, mappingName, cfgsection string, m *model.Model, fgin map[string]string, id eh.UUID) breadcrumb {
	mm := ModelMappings{
		cfgsect:        cfgsection,
		model:          m,
		mappings:       []mapping{},
		requestedFQDD:  fgin["FQDD"],
		requestedGroup: fgin["Group"],
		requestedIndex: fgin["Index"],
		loaded:         false,
	}

	ars.mappingsMu.Lock()
	ars.modelmappings[mappingName] = mm
	ars.mappingsMu.Unlock()
	ars.hashMu.Lock()
	ars.hashDirty = true
	ars.hashMu.Unlock()

	return breadcrumb{ars: ars, mappingName: mappingName}
}

func (b breadcrumb) Close() {
	b.ars.mappingsMu.Lock()
	delete(b.ars.modelmappings, b.mappingName)
	b.ars.mappingsMu.Unlock()

	b.ars.logger.Info("CLOSE on breadcrumb. SETTING HASH DIRTY", "mappingName", b.mappingName)
	b.ars.hashMu.Lock()
	b.ars.hashDirty = true
	b.ars.hashMu.Unlock()
}

func (b breadcrumb) UpdateRequest(ctx context.Context, property string, value interface{}, auth *domain.RedfishAuthorizationProperty) (interface{}, error) {
	b.ars.mappingsMu.RLock()
	needsUnlock := true
	// defer so that we unlock on panic and dont lock everything up, but need to support unlock earlier
	defer func() {
		if needsUnlock {
			b.ars.mappingsMu.RUnlock()
		}
	}()

	canned_response := `{"RelatedProperties@odata.count": 1, "Message": "%s", "MessageArgs": ["%[2]s"], "Resolution": "Remove the %sproperty from the request body and resubmit the request if the operation failed.", "MessageId": "%s", "MessageArgs@odata.count": 1, "RelatedProperties": ["%[2]s"], "Severity": "Warning"}`
	timeout_response := `{"RelatedProperties@odata.count": 0, "Message": "Request Timed Out", "MessageArgs": [], "Resolution": "", "MessageId": "", "MessageArgs@odata.count": 0, "RelatedProperties": [], "Severity": "Error"}`
	b.ars.logger.Info("UpdateRequest", "property", property, "mappingName", b.mappingName)
	num_success := 0
	found_flag := false

	reqIDs := []eh.UUID{}
	responses := []a.AttributeUpdatedData{}
	errs := []string{}
	patch_timeout := 10
	//patch_timeout := 3000

	listener := eventwaiter.NewListener(ctx, b.ars.logger, b.ars.ew, func(event eh.Event) bool {
		if event.EventType() != a.AttributeUpdated {
			return false
		}
		data, ok := event.Data().(*a.AttributeUpdatedData)
		if !ok {
			return false
		}
		if data.Name == "SledProfile" {
			return false
		}
		return true
	})
	listener.Name = "ar patch listener"
	defer listener.Close()

	mappings, ok := b.ars.modelmappings[b.mappingName]
	if !ok {
		return nil, errors.New("Could not find mapping: " + b.mappingName)
	}

	for _, mapping := range mappings.mappings {

		if property != mapping.Property {
			continue
		}

		var ad a.AttributeData
		if !ad.WriteAllowed(property, auth) {
			b.ars.logger.Error("Unable to set", "Attribute", property)
			err_msg := fmt.Sprintf("The property %s is a read only property and cannot be assigned a value.", property)
			errs = append(errs, fmt.Sprintf(canned_response, []interface{}{err_msg, property, "", "Base.1.0.PropertyNotWritable"}...))
			continue
		}

		b.ars.logger.Info("Sending Update Request", "mapping", mapping, "value", value)
		reqUUID := eh.NewUUID()

		data := &a.AttributeUpdateRequestData{
			ReqID:         reqUUID,
			FQDD:          mapping.FQDD,
			Group:         mapping.Group,
			Index:         mapping.Index,
			Name:          mapping.Name,
			Value:         value,
			Authorization: *auth,
		}
		b.ars.eb.PublishEvent(ctx, eh.NewEvent(a.AttributeUpdateRequest, data, time.Now()))
		reqIDs = append(reqIDs, reqUUID)
		found_flag = true
		break
	}

	// we could be locked a very long time if this listener is slow, so unlock now and tell the defer not to bother
	b.ars.mappingsMu.RUnlock()
	needsUnlock = false

	if !found_flag {
		b.ars.logger.Error("not found", "Attribute", property)
		err_msg := fmt.Sprintf("The property %s is not in the list of valid properties for the resource.", property)
		errs = append(errs, fmt.Sprintf(canned_response, []interface{}{err_msg, property, "unknown ", "Base.1.0.PropertyUnknown"}...))
	}

	if len(reqIDs) == 0 {
		return nil, domain.HTTP_code{Err_message: errs, Any_success: num_success}
	}

	// set up ret for default (timeout). If ProcessEvents() doesn't reset ret to a succes, this is what goes out
	ret := domain.HTTP_code{Err_message: []string{timeout_response}, Any_success: num_success}
	timedCtx, cancel := context.WithTimeout(ctx, time.Duration(patch_timeout*len(reqIDs))*time.Second)
	defer cancel()
	listener.ProcessEvents(timedCtx, func(event eh.Event) {
		data, ok := event.Data().(*a.AttributeUpdatedData)
		if !ok {
			return
		}
		for i, reqID := range reqIDs {
			if reqID == data.ReqID {
				reqIDs[i] = reqIDs[len(reqIDs)-1]
				reqIDs = reqIDs[:len(reqIDs)-1]
				responses = append(responses, *data)
				if data.Error != "" {
					errs = append(errs, data.Error)
				} else {
					num_success = num_success + 1
				}
				break
			}
		}
		if len(reqIDs) == 0 {
			ret = domain.HTTP_code{Err_message: errs, Any_success: num_success}
			cancel()
		}
	})

	return nil, ret
}

func (ars *ARService) optimizeHash() {
	ars.mappingsMu.Lock()
	defer ars.mappingsMu.Unlock()
	for k := range ars.modelmappings {
		ars.loadConfig(k)
	}

	ars.hashMu.Lock()
	defer ars.hashMu.Unlock()

	// clear out old optimized hash in preparation
	for k := range ars.hash {
		delete(ars.hash, k)
	}

	for _, mapping := range ars.modelmappings {
		for _, mm := range mapping.mappings {
			mapstring := fmt.Sprintf("%s:%s:%s:%s", mm.FQDD, mm.Group, mm.Index, mm.Name)
			updArr, ok := ars.hash[mapstring]
			if !ok {
				updArr = []update{}
			}
			updArr = append(updArr, update{model: mapping.model, property: mm.Property})
			ars.hash[mapstring] = updArr
		}
	}
	ars.hashDirty = false
	ars.logger.Info("finished optimizing hash", "len(hash)", len(ars.hash), "hash", ars.hash)
}

// loadConfig must be called with ars.mappingsMu!
func (ars *ARService) loadConfig(mappingName string) {
	if ars.modelmappings[mappingName].loaded {
		return
	}

	ars.logger.Info("Updating Config")

	ars.cfgMu.Lock()
	defer ars.cfgMu.Unlock()

	subCfg := ars.cfg.Sub("mappings")
	if subCfg == nil {
		ars.logger.Warn("missing config file section: 'mappings'")
		return
	}

	newmaps := []mapping{}
	err := subCfg.UnmarshalKey(ars.modelmappings[mappingName].cfgsect, &newmaps)
	if err != nil {
		ars.logger.Warn("Unmarshal failed", "err", err)
	}

	ars.logger.Info("Loading Config", "mappingName", mappingName, "configsection", ars.modelmappings[mappingName].cfgsect, "mappings", newmaps)

	modelmapping := ars.modelmappings[mappingName]

	realmaps := make([]mapping, 0, len(newmaps))
	for _, mm := range newmaps {
		newentry := mm
		if newentry.FQDD == "{FQDD}" {
			ars.logger.Debug("Replacing {FQDD} with real fqdd", "fqdd", newentry.FQDD, "real_fqdd", modelmapping.requestedFQDD)
			newentry.FQDD = modelmapping.requestedFQDD
		}
		if newentry.Group == "{GROUP}" {
			ars.logger.Debug("Replacing {GROUP} with real group", "group", newentry.Group, "real_group", modelmapping.requestedGroup)
			newentry.Group = modelmapping.requestedGroup
		}
		if newentry.Index == "{INDEX}" {
			ars.logger.Debug("Replacing {INDEX} with real index", "index", newentry.Index, "real_index", modelmapping.requestedIndex)
			newentry.Index = modelmapping.requestedIndex
		}
		realmaps = append(realmaps, newentry)
	}
	modelmapping.mappings = realmaps
	modelmapping.loaded = true
	ars.modelmappings[mappingName] = modelmapping

	ars.logger.Info("Loaded Config", "mappingName", mappingName, "configsection", ars.modelmappings[mappingName].cfgsect, "mappings", ars.modelmappings[mappingName].mappings)
}
