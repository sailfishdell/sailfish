package awesome_mapper2

import (
	"errors"
	"fmt"
	"github.com/superchalupa/sailfish/godefs"
	"strconv"
	"sync"
	"time"

	"github.com/Knetic/govaluate"
	eh "github.com/looplab/eventhorizon"

	a "github.com/superchalupa/sailfish/src/dell-resources/attributedef"
	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
	"github.com/superchalupa/sailfish/src/log"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type setupProcessFunc func(log.Logger, interface{}) (processFunc, processSetupFunc, error)

type processFunc func(p *MapperParameters, e eh.Event, ch eh.CommandHandler, d *domain.DomainObjects) error
type processSetupFunc func(p *MapperParameters) error

var setupProcessFuncsInit sync.Once
var setupProcessFuncsMu sync.RWMutex
var setupProcessFuncs map[string]setupProcessFunc

func InitAM3ProcessSetupFunctions() (map[string]setupProcessFunc, *sync.RWMutex) {
	setupProcessFuncsInit.Do(func() { setupProcessFuncs = map[string]setupProcessFunc{} })
	return setupProcessFuncs, &setupProcessFuncsMu
}

func AddAM3ProcessSetupFunction(name string, fn setupProcessFunc) {
	InitAM3ProcessSetupFunctions()
	setupProcessFuncsMu.Lock()
	setupProcessFuncs[name] = fn
	setupProcessFuncsMu.Unlock()
}

type ModelUpdate struct {
	property    string
	queryString string                         // am2
	queryExpr   *govaluate.EvaluableExpression //am2
	defaultVal  interface{}
	//eventMem  string   	//am3
	//actionFn interface{} 	// am3
}

//func getEventMember(event eh.Event, member String){
//	r := reflect.ValueOf(event.Data())
//	if !ok {
//		return nil,errors.New("not present in event", "member", member, "event data", event.Data())
//	}
//	return f, nil
//}

func init() {
	AddAM3ProcessSetupFunction("govaluate_modelupdate", func(logger log.Logger, modelUpdates interface{}) (processFunc, processSetupFunc, error) {
		mus, ok := modelUpdates.([]*ConfigFileModelUpdate)
		if !ok {
			return nil, nil, errors.New("govaluate_modelupdate is missing []*ConfigFileModelUpdate parameter")
		}

		// initial parsing and setup
		m := []*ModelUpdate{}
		for _, modelUpdate := range mus {
			// this queryExpr would be handled the same as a exec expr
			queryExpr, err := govaluate.NewEvaluableExpressionWithFunctions(modelUpdate.Query, functions)
			if err != nil {
				logger.Crit("Query construction failed", "query", modelUpdate.Query, "err", err)
				continue
			}

			m = append(m, &ModelUpdate{
				property:    modelUpdate.Property,
				queryString: modelUpdate.Query,
				queryExpr:   queryExpr,
				defaultVal:  modelUpdate.Default,
			})
		}

		// Model Update Function
		modelUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d *domain.DomainObjects) error {

			for _, updates := range m {
				mp.model.StopNotifications()
				mp.Params["propname"] = updates.property
				val, err := updates.queryExpr.Evaluate(mp.Params)
				if err != nil {
					logger.Error("Expression failed to evaluate", "err", err, "parameters", mp.Params, "val", val)
					continue
				}
				// comment out logging in the fast path. uncomment to enable.
				//ret.logger.Info("Updating property!", "property", updates.property, "value", val, "Event", event, "EventData", event.Data())
				mp.model.UpdateProperty(updates.property, val)
				mp.model.NotifyObservers()
				mp.model.StartNotifications()
			}

			delete(mp.Params, "propname")
			return nil
		}

		modelDefaultSetupFn := func(mp *MapperParameters) error {

			if mp.model == nil {
				return errors.New("parameters model is nil")
			}

			mp.model.StopNotifications()
			defer mp.model.NotifyObservers()
			defer mp.model.StartNotifications()

			for _, mapperUpdate := range m {
				if mapperUpdate.defaultVal != nil {
					mp.model.UpdateProperty(mapperUpdate.property, mapperUpdate.defaultVal)
				} else {
					mp.model.UpdateProperty(mapperUpdate.property, "")
				}
			}
			return nil
		}
		return modelUpdateFn, modelDefaultSetupFn, nil

	})
	AddAM3ProcessSetupFunction("govaluate_exec", func(logger log.Logger, execs interface{}) (processFunc, processSetupFunc, error) {
		// initial parsing and setup
		es, ok := execs.([]string)
		if !ok {
			return nil, nil, errors.New("govaluate_exec is missing []string parameter")

		}
		execExprSlice := []*govaluate.EvaluableExpression{}
		for _, exec := range es {
			execExpr, err := govaluate.NewEvaluableExpressionWithFunctions(exec, functions)
			if err != nil {
				logger.Crit("Query construction failed", "exec", exec, "err", err)
				continue
			}
			execExprSlice = append(execExprSlice, execExpr)
		}

		return func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d *domain.DomainObjects) error {
			for _, ee := range execExprSlice {
				val, err := ee.Evaluate(mp.Params)
				if err != nil {
					logger.Error("Expression failed to evaluate", "parameters", mp.Params, "val", val)
					continue
				}
			}
			return nil
		}, nil, nil
	})

	AddAM3ProcessSetupFunction("updateFanData", func(logger log.Logger, processConfig interface{}) (processFunc, processSetupFunc, error) {
		// Model Update Function
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d *domain.DomainObjects) error {

			dmobj, ok := event.Data().(*godefs.DMObject)
			fanobj, ok := dmobj.Data.(*godefs.DM_thp_fan_data_object)
			if !ok {
				logger.Error("updateFanData did not have fan event", "type", event.EventType, "data", event.Data())
				return errors.New("updateFanData did not receive FanEventData")
			}

			ch.HandleCommand(mp.ctx,
				&domain.UpdateRedfishResourceProperties2{
					ID: mp.uuid,
					Properties: map[string]interface{}{
						"Reading":     (fanobj.Rotor1rpm + fanobj.Rotor2rpm) / 2,
						"Oem/Reading": fanobj.Int,
					},
				})
			return nil
		}

		return aggUpdateFn, nil, nil
	})

	AddAM3ProcessSetupFunction("updatePwrConsumptionData", func(logger log.Logger, processConfig interface{}) (processFunc, processSetupFunc, error) {
		// Model Update Function
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d *domain.DomainObjects) error {
			data, ok := event.Data().(*dm_event.PowerConsumptionDataObjEventData)
			if !ok {
				logger.Error("updatePwrConsumptionData not have PowerConsumptionDataObjEvent event", "type", event.EventType, "data", event.Data())
				return errors.New("updatePwrConsumptionData did not receive PowerConsumptionDataObjEventData")
			}
			ch.HandleCommand(mp.ctx,
				&domain.UpdateRedfishResourceProperties2{
					ID: mp.uuid,
					Properties: map[string]interface{}{
						"Oem/EnergyConsumptionStartTime": data.CwStartTime,
						"Oem/EnergyConsumptionkWh":       int(data.CumulativeWatts / 1000),
						"Oem/MaxPeakWatts":               data.PeakWatts,
						"Oem/MaxPeakWattsTime":           epoch2Date(data.PwReadingTime),
						"Oem/MinPeakWatts":               data.MinWatts,
						"Oem/MinPeakWattsTime":           epoch2Date(data.MinwReadingTime),
						"Oem/PeakHeadroomWatts":          data.PeakHeadRoom,
						"PowerConsumedWatts":             data.InstWattsPSU1_2,
						"PowerAvailableWatts":            data.PeakHeadRoom,
					},
				})

			return nil
		}

		return aggUpdateFn, nil, nil
	})

	AddAM3ProcessSetupFunction("updatePwrSupplyData", func(logger log.Logger, processConfig interface{}) (processFunc, processSetupFunc, error) {
		// Model Update Function
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d *domain.DomainObjects) error {
			data, ok := event.Data().(*dm_event.PowerSupplyObjEventData)
			if !ok {
				logger.Error("updatePowerSupplyObjEvent does not have PowerSupplyObjEvent event", "type", event.EventType, "data", event.Data())
				return errors.New("updatePowerSupplyObjEvent did not receive PowerSupplyObjEvent")
			}

			inputvolts := zero2null(data.CurrentInputVolts)
			inputcurrent := round2DecPlaces(data.InstAmps, true)
			health := get_health(data.ObjectHeader.ObjStatus)

			ch.HandleCommand(mp.ctx,
				&domain.UpdateRedfishResourceProperties2{
					ID: mp.uuid,
					Properties: map[string]interface{}{
						"LineInputVoltage":      inputvolts,
						"Oem/Dell/InputCurrent": inputcurrent,
						"Status/HealthRollup":   health,
						"Status/Health":         health,
					},
				})
			return nil
		}

		return aggUpdateFn, nil, nil
	})

	AddAM3ProcessSetupFunction("updateIOMConfigData", func(logger log.Logger, processConfig interface{}) (processFunc, processSetupFunc, error) {
		// Model Update Function
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d *domain.DomainObjects) error {
			data, ok := event.Data().(*dm_event.IomCapabilityData)
			if !ok {
				logger.Error("updateIOMConfigData does not have IOMCapabilityData event", "type", event.EventType, "data", event.Data())
				return errors.New("updateIOMConfigData did not receive IOMCapabilityData")
			}
			ch.HandleCommand(mp.ctx,
				&domain.UpdateRedfishResourceProperties2{
					ID: mp.uuid,
					Properties: map[string]interface{}{
						"internal_mgmt_supported":  data.Internal_mgmt_supported,
						"IOMConfig_objects":        data.IOMConfig_objects,
						"Capabilities":             data.Capabilities,
						"Capabilities@odata.count": data.CapabilitiesCount,
					},
				})
			return nil
		}

		return aggUpdateFn, nil, nil
	})

	powercap_enabled := false
	AddAM3ProcessSetupFunction("updatePowerCapFlag", func(logger log.Logger, processConfig interface{}) (processFunc, processSetupFunc, error) {
		// Model Update Function
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d *domain.DomainObjects) error {
			data, ok := event.Data().(*a.AttributeUpdatedData)
			if !ok {
				logger.Error("updatePowerCapFlag does not have AttributeUpdated event", "type", event.EventType, "data", event.Data())
				return errors.New("updatePowerCapFlag did not receive AttributeUpdated event")
			}

			if data.Value == "Enabled" {
				powercap_enabled = true
			} else {
				powercap_enabled = false
			}
			return nil
		}

		return aggUpdateFn, nil, nil
	})

	AddAM3ProcessSetupFunction("updatePowerLimit", func(logger log.Logger, processConfig interface{}) (processFunc, processSetupFunc, error) {
		// Model Update Function
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d *domain.DomainObjects) error {
			data, ok := event.Data().(*a.AttributeUpdatedData)
			if !ok {
				logger.Error("updatePowerLimit does not have AttributeUpdated event", "type", event.EventType, "data", event.Data())
				return errors.New("updatePowerLimit not receive AttributeUpdated")
			}
			powerlimit := 0

			if powercap_enabled == true {
				powerlimit, ok = data.Value.(int)
				if !ok {
					return errors.New("power limit is not an integer")
				}
			}

			ch.HandleCommand(mp.ctx,
				&domain.UpdateRedfishResourceProperties2{
					ID: mp.uuid,
					Properties: map[string]interface{}{
						"PowerLimit/LimitInWatts": powerlimit,
					},
				})

			return nil
		}
		return aggUpdateFn, nil, nil
	})

}

func round2DecPlaces(value float64, nilFlag bool) interface{} {
	if nilFlag == true && value == 0 {
		return nil
	}

	vs := fmt.Sprintf("%.2f", value)
	val, err := strconv.ParseFloat(vs, 2)
	if err != nil {
		return value
	}
	return val
}
func epoch2Date(date int64) time.Time {
	return time.Unix(date, 0)
}

func zero2null(value int) interface{} {
	if value == 0 {
		return nil
	} else {
		return value
	}
}
func get_health(health int) interface{} {

	switch health {
	case 0, 1: //other, unknown
		return nil
	case 2: //ok
		return "OK"
	case 3: //non-critical
		return "Warning"
	case 4, 5: //critical, non-recoverable
		return "Critical"
	default:
		return nil
	}
}
