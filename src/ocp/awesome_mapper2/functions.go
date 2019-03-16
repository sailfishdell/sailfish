package awesome_mapper2

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Knetic/govaluate"
	"github.com/superchalupa/sailfish/src/ocp/model"
)

var functionsInit sync.Once
var functionsMu sync.RWMutex
var functions map[string]govaluate.ExpressionFunction

func InitFunctions() (map[string]govaluate.ExpressionFunction, *sync.RWMutex) {
	functionsInit.Do(func() { functions = map[string]govaluate.ExpressionFunction{} })
	return functions, &functionsMu
}

func AddFunction(name string, fn func(args ...interface{}) (interface{}, error)) {
	InitFunctions()
	functionsMu.Lock()
	functions[name] = fn
	functionsMu.Unlock()
}

func CompareURLStrings(strA, strB string) bool {
	a, err := strconv.Atoi(path.Base(strA))
	if err != nil {
		a = 0
	}
	b, err := strconv.Atoi(path.Base(strB))
	if err != nil {
		b = 0
	}
	return a > b
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
			mu := args[1].(*sync.RWMutex)
			key := reflect.ValueOf(args[2])
			value := reflect.ValueOf(args[3])

			mu.Lock()
			hash.SetMapIndex(key, value)
			mu.Unlock()

			return args[2], nil
		})
	AddFunction("map_chassis_state", func(args ...interface{}) (interface{}, error) {
		switch t := args[0].(string); t {
		case "Chassis Standby Power State":
			return "Off", nil
		case "Chassis Power On State":
			return "On", nil
		case "Chassis Powering On State":
			return "PoweringOn", nil
		case "Chassis Powering Off State":
			return "PoweringOff", nil
		default:
			return nil, nil
		}
	})

	AddFunction("int", func(args ...interface{}) (interface{}, error) {
		switch t := args[0].(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
			return int(reflect.ValueOf(t).Int()), nil
		case float32, float64:
			return int(reflect.ValueOf(t).Float()), nil
		case string:
			float, err := strconv.ParseFloat(t, 64)
			return int(float), err
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

		var vStr []string

		switch t := v.(type) {
		case []string:
			vStr = t
		case []interface{}:
			for _, i := range t {
				if k, ok := i.(string); ok {
					vStr = append(vStr, k)
				}
			}
		default:
			vStr = []string{}
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
	AddFunction("reversesorttoset", func(args ...interface{}) (interface{}, error) {
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

		var vStr []string

		switch t := v.(type) {
		case []string:
			vStr = t
		case []interface{}:
			for _, i := range t {
				if k, ok := i.(string); ok {
					vStr = append(vStr, k)
				}
			}
		default:
			vStr = []string{}
		}

		found := false
		for i := range vStr {
			if vStr[i] == str {
				found = true
			}
		}
		if !found {
			if len(vStr) == 0 {
				vStr = append(vStr, str)
			} else if CompareURLStrings(str, vStr[0]) {
				vStr = append([]string{str}, vStr...)
			} else if CompareURLStrings(vStr[len(vStr)-1], str) {
				vStr = append(vStr, str)
			} else {
				index := sort.Search(len(vStr), func(i int) bool { return CompareURLStrings(str, vStr[i]) })
				vStr = append(vStr, "")
				copy(vStr[index+1:], vStr[index:])
				vStr[index] = str
			}
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
	AddFunction("hassuffix", func(args ...interface{}) (interface{}, error) {
		str, ok := args[0].(string)
		if !ok {
			return nil, errors.New("expected a string argument")
		}
		suffix, ok := args[1].(string)
		if !ok {
			return nil, errors.New("expected a string argument")
		}
		return strings.HasSuffix(str, suffix), nil
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
	AddFunction("round_2_dec_pl", func(args ...interface{}) (interface{}, error) {
		val, ok := args[0].(float64)
		var err error

		if !ok {
			return args[0], nil
		}

		valStr := fmt.Sprintf("%.2f", val)
		val, err = strconv.ParseFloat(valStr, 2)
		if err != nil {
			return args[0], nil
		}

		return val, nil
	})
	AddFunction("zero_to_null", func(args ...interface{}) (interface{}, error) {
		switch args[0].(type) {
		case int, int8, int16, int32, int64:
			val := int(args[0].(float64))
			if val == 0 {
				return nil, nil
			}
			return val, nil
		case float32, float64:
			val := args[0].(float64)
			if val == 0 {
				return nil, nil
			}
			return val, nil
		default:
			return nil, errors.New("cant parse non-int or non-float")
		}
	})
	AddFunction("zero_or_value", func(args ...interface{}) (interface{}, error) {
		switch t := args[0].(float64); t {
		default:
			if t < 0 {
				return 0, nil
			} else {
				return t, nil
			}
		}
	})
	AddFunction("null_lt_zero", func(args ...interface{}) (interface{}, error) {
		if args[0] == 0 {
			return nil, nil
		}
		switch t := args[0].(float64); t {
		default:
			if t < 0 {
				return nil, nil
			} else {
				return t, nil
			}
		}
	})
	AddFunction("empty_to_null", func(args ...interface{}) (interface{}, error) {
		if args[0] == "" {
			return nil, nil
		}
		return args[0], nil
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
