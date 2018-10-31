package awesome_mapper2

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Knetic/govaluate"
	"github.com/superchalupa/sailfish/src/ocp/model"
)

var functionsInit sync.Once
var functions map[string]govaluate.ExpressionFunction

func InitFunctions() map[string]govaluate.ExpressionFunction {
	functionsInit.Do(func() { functions = map[string]govaluate.ExpressionFunction{} })
	return functions
}

func AddFunction(name string, fn func(args ...interface{}) (interface{}, error)) {
	InitFunctions()
	functions[name] = fn
}

func init() {
	InitFunctions()

	// debugging function
	AddFunction("echo", func(args ...interface{}) (interface{}, error) {
		fmt.Println(args...)
		return true, nil
	})

	AddFunction("array",
		func(args ...interface{}) (interface{}, error) {
			a := []interface{}{}
			for _, i := range args {
				a = append(a, i)
			}
			return a, nil
		})

	AddFunction("set_hash_value",
		func(args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, errors.New("set_hash_value failed, not enough arguments")
			}
			hash := reflect.ValueOf(args[0])
			key := reflect.ValueOf(args[1])
			value := reflect.ValueOf(args[2])
			hash.SetMapIndex(key, value)

			return args[2], nil
		})

	AddFunction("int", func(args ...interface{}) (interface{}, error) {
		switch t := args[0].(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
			return float64(reflect.ValueOf(t).Int()), nil
		case float32, float64:
			return float64(reflect.ValueOf(t).Float()), nil
		case string:
			float, err := strconv.ParseFloat(t, 64)
			return float, err
		default:
			return nil, errors.New("cant parse non-string")
		}
	})
	AddFunction("removefromset", func(args ...interface{}) (interface{}, error) {
		model, ok := args[0].(*model.Model)
		if !ok {
			return nil, errors.New("need model as first arg")
		}
		property, ok := args[1].(string)
		if !ok {
			return nil, errors.New("need property name as second arg")
		}
		str, ok := args[2].(string)
		if !ok {
			return nil, errors.New("need new value as third arg")
		}
		v, ok := model.GetPropertyOk(property)
		if !ok || v == nil {
			v = []string{}
		}
		vStr, ok := v.([]string)
		if !ok {
			v = []string{}
			vStr = v.([]string)
		}

		ret := []string{}
		for i := range vStr {
			if vStr[i] == str {
				ret = vStr[:i]
				if i+1 < len(vStr) {
					ret = append(ret, vStr[i+1:]...)
				}
				break
			}
		}
		return ret, nil
	})
	AddFunction("addtoset", func(args ...interface{}) (interface{}, error) {
		model, ok := args[0].(*model.Model)
		if !ok {
			return nil, errors.New("need model as first arg")
		}
		property, ok := args[1].(string)
		if !ok {
			return nil, errors.New("need property name as second arg")
		}
		str, ok := args[2].(string)
		if !ok {
			return nil, errors.New("need new value as third arg")
		}
		v, ok := model.GetPropertyOk(property)
		if !ok || v == nil {
			v = []string{}
		}
		vStr, ok := v.([]string)
		if !ok {
			v = []string{}
			vStr = v.([]string)
		}

		found := false
		for i := range vStr {
			if vStr[i] == str {
				found = true
			}
		}
		if !found {
			vStr = append(vStr, str)
		}
		return vStr, nil
	})
	AddFunction("nohash", func(args ...interface{}) (interface{}, error) {
		str, ok := args[0].(string)
		if !ok {
			return nil, errors.New("expected a string argument")
		}
		if i := strings.Index(str, "#"); i > -1 {
			return false, nil
		}
		return true, nil
	})
	AddFunction("baseuri", func(args ...interface{}) (interface{}, error) {
		str, ok := args[0].(string)
		if !ok {
			return nil, errors.New("expected a string argument")
		}
		return path.Dir(str), nil
	})
	AddFunction("strlen", func(args ...interface{}) (interface{}, error) {
		length := len(args[0].(string))
		return (float64)(length), nil
	})
	AddFunction("epoch_to_date", func(args ...interface{}) (interface{}, error) {
		return time.Unix(int64(args[0].(float64)), 0), nil
	})
	AddFunction("traverse_struct", func(args ...interface{}) (interface{}, error) {
		s := args[0]
		for _, name := range args[1:] {
			n := name.(string)
			r := reflect.ValueOf(s)
			s = reflect.Indirect(r).FieldByName(n).Interface()
		}

		// have to return float64 for all numeric types
		switch t := s.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
			return float64(reflect.ValueOf(t).Int()), nil
		case float32, float64:
			return float64(reflect.ValueOf(t).Float()), nil
		default:
			return s, nil
		}
	})
  AddFunction("map_health_value", func(args ...interface{}) (interface{}, error) { //todo: turn into hash
		switch t := args[0].(float64); t {
		case 0, 1: //other, unknown
			return nil, nil
		case 2: //ok
			return "OK", nil
		case 3: //non-critical
			return "Warning", nil
		case 4, 5: //critical, non-recoverable
			return "Critical", nil
		default:
			return nil, errors.New("Invalid object status")
		}
	})
  AddFunction("map_led_value", func(args ...interface{}) (interface{}, error) { //todo: turn into hash
    switch t := args[0].(string); t {
    case "Blink-Off", "BLINK-OFF":
      return "Lit", nil
    case "Blink-1", "Blink-2", "BLINK-ON":
      return "Blinking", nil
    default:
      return nil, nil
    }
  })
	AddFunction("string", func(args ...interface{}) (interface{}, error) {
		switch t := args[0].(type) {
		case int, int8, int16, int32, int64:
			str := strconv.FormatInt(reflect.ValueOf(t).Int(), 10)
			return str, nil
		case uint, uint8, uint16, uint32, uint64:
			str := strconv.FormatUint(reflect.ValueOf(t).Uint(), 10)
			return str, nil
		case float32, float64:
			str := strconv.FormatFloat(reflect.ValueOf(t).Float(), 'G', -1, 64)
			return str, nil
		case string:
			return t, nil
		default:
			return nil, errors.New("Not an int, float, or string")
		}
	})
	AddFunction("zero_to_null", func(args ...interface{}) (interface{}, error) {
		if args[0] == 0 {
			return nil, nil
		}
		return args[0], nil
	})
	AddFunction("subsystem_health", func(args ...interface{}) (interface{}, error) {
		fqdd := strings.Split(args[0].(map[string]string)["FQDD"], "#")
		subsys := fqdd[len(fqdd)-1]
		health := args[0].(map[string]string)["Health"]
		if health == "Absent" {
			return nil, nil
		}
		return map[string]string{"subsys": subsys, health: "health"}, nil
	})
	AddFunction("read_file", func(args ...interface{}) (interface{}, error) {
		lines := ""
		file_path := args[0].(string)
		if file_path == "NONE" {
			return nil, nil
		}
		bytes, err := ioutil.ReadFile(file_path)
		if err != nil {
			return nil, err
		} else {
			lines = string(bytes)
		}
		err = os.Remove(file_path)
		if err != nil {
			return lines, err
		}
		return lines, nil
	})
}
