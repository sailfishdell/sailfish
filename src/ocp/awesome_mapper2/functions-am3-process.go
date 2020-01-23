package awesome_mapper2

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Knetic/govaluate"
	eh "github.com/looplab/eventhorizon"

	a "github.com/superchalupa/sailfish/src/dell-resources/attributedef"
	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type setupProcessFunc func(log.Logger, interface{}) (processFunc, processSetupFunc, error)

type BusObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
	GetCommandHandler() eh.CommandHandler
}

type processFunc func(p *MapperParameters, e eh.Event, ch eh.CommandHandler, d BusObjs) error
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

const CONV_FUNC_KEY = "Conv"
const CONV_VALUE_KEY = "Field"

type convFunc func(value interface{}) (interface{}, bool)
type setupConvFunc func(logger log.Logger) convFunc

var setupConvFuncsInit sync.Once
var setupConvFuncsMu sync.RWMutex
var setupConvFuncs map[string]setupConvFunc

func InitAM3ConversionFunctions() (map[string]setupProcessFunc, *sync.RWMutex) {
	setupConvFuncsInit.Do(func() { setupConvFuncs = map[string]setupConvFunc{} })
	return setupProcessFuncs, &setupProcessFuncsMu
}

func AddAM3ConversionFunction(name string, fn setupConvFunc) {
	InitAM3ConversionFunctions()
	setupConvFuncsMu.Lock()
	setupConvFuncs[name] = fn
	setupConvFuncsMu.Unlock()
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
		modelUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d BusObjs) error {

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

		return func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d BusObjs) error {
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

	AddAM3ProcessSetupFunction("updatePwrSupplyData", func(logger log.Logger, processConfig interface{}) (processFunc, processSetupFunc, error) {
		// Model Update Function
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d BusObjs) error {
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

	// should not be used until IOMConfig_objects and Capabilities are ironed out
	AddAM3ProcessSetupFunction("updateIOMConfigData", func(logger log.Logger, processConfig interface{}) (processFunc, processSetupFunc, error) {
		// Model Update Function
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d BusObjs) error {
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
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d BusObjs) error {
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
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d BusObjs) error {
			data, ok := event.Data().(*a.AttributeUpdatedData)
			if !ok {
				logger.Error("updatePowerLimit does not have AttributeUpdated event", "type", event.EventType, "data", event.Data())
				return errors.New("updatePowerLimit not receive AttributeUpdated")
			}
			powerlimit := 0

			if powercap_enabled {
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

	AddAM3ProcessSetupFunction("am3AttributeUpdated", func(logger log.Logger, processConfig interface{}) (processFunc, processSetupFunc, error) {
		aggUpdateFn := func(mp *MapperParameters, event eh.Event, ch eh.CommandHandler, d BusObjs) error {
			data, ok := event.Data().(*a.AttributeUpdatedData)
			if !ok {
				logger.Error("Did not have AttributeUpdated event", "type", event.EventType, "data", event.Data())
				return errors.New("did not receive AttributeUpdated")
			}

			//logger.Error("Received AttributeUpdatedData event", "value",data.Value)

			// crash if these don't work as it is a confuration error and needs to be fixed
			param := processConfig.(map[interface{}]interface{})
			key := param[CONV_VALUE_KEY].(string)

			// use the attribute value unless a conversion function has been specified
			val := data.Value

			// the conversion funcation is optional
			if helpFunc, ok := param[CONV_FUNC_KEY].(string); ok {
				val, ok = setupConvFuncs[helpFunc](logger)(data.Value)
				if !ok {
					logger.Error("data", "value", val, "parsed", ok)
				}
			}

			ch.HandleCommand(mp.ctx,
				&domain.UpdateRedfishResourceProperties2{
					ID: mp.uuid,
					Properties: map[string]interface{}{
						key: val,
					},
				})

			return nil
		}

		return aggUpdateFn, nil, nil
	})

	/*
	                       AM3 Conversion functions

	   Logic functions to convert the backend data format into a Redfish Spec compliant
	   format that can be consumed.
	*/

	AddAM3ConversionFunction("value_to_string", func(logger log.Logger) convFunc {
		convFn := func(value interface{}) (interface{}, bool) {
			logger.Debug("AM3 conversion", "value", value)

			switch t := value.(type) {
			case uint, uint8, uint16, uint32, uint64:
				str := strconv.FormatUint(reflect.ValueOf(t).Uint(), 10)
				return str, true
			case float32, float64:
				str := strconv.FormatFloat(reflect.ValueOf(t).Float(), 'G', -1, 64)
				return str, true
			case string:
				return t, true
			case int, int8, int16, int32, int64:
				str := strconv.FormatInt(reflect.ValueOf(t).Int(), 10)
				return str, true
			default:
				return nil, false
			}

		}

		return convFn
	})

	AddAM3ConversionFunction("empty_to_null", func(logger log.Logger) convFunc {
		convFn := func(value interface{}) (interface{}, bool) {
			logger.Debug("AM3 conversion", "value", value)

			if value == "" {
				return nil, true
			}
			return value, true
		}

		return convFn
	})

	AddAM3ConversionFunction("map_task_state", func(logger log.Logger) convFunc {
		convFn := func(value interface{}) (interface{}, bool) {
			logger.Debug("AM3 conversion", "value", value)

			task_state, ok := value.(string)
			if strings.EqualFold(task_state, "Completed") {
				return "Completed", ok
			} else if strings.EqualFold(task_state, "Interrupted") {
				return "Interrupted", ok
			}

			// default to "Running"
			return "Running", ok
		}

		return convFn
	})

	AddAM3ConversionFunction("map_power_state", func(logger log.Logger) convFunc {
		convFn := func(value interface{}) (interface{}, bool) {
			logger.Debug("AM3 conversion", "value", value)

			switch t, ok := value.(string); t {
			case "Chassis Standby Power State":
				return "Off", ok
			case "Chassis Power On State":
				return "On", ok
			case "Chassis Powering On State":
				return "PoweringOn", ok
			case "Chassis Powering Off State":
				return "PoweringOff", ok
			default:
				return "", ok
			}
		}

		return convFn
	})

}

func round2DecPlaces(value float64, nilFlag bool) interface{} {
	msm_flag := false
	if nilFlag && value == 0 {
		return nil
	}

	if msm_flag {
		return value
	}

	vs := fmt.Sprintf("%.2f", value)
	val, err := strconv.ParseFloat(vs, 2)
	if err != nil {
		return value
	}
	return val
}
func epoch2Date(date int64) string {
	return time.Unix(date, 0).String()
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
