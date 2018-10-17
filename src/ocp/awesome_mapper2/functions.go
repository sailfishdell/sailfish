package awesome_mapper2

import (
	"errors"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Knetic/govaluate"
	"github.com/superchalupa/sailfish/src/ocp/model"
)

var functions map[string]govaluate.ExpressionFunction

func init() {
	functions = map[string]govaluate.ExpressionFunction{
		"int": func(args ...interface{}) (interface{}, error) {
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
		},
		"removefromset": func(args ...interface{}) (interface{}, error) {
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
		},
		"addtoset": func(args ...interface{}) (interface{}, error) {
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
		},
		"nohash": func(args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, errors.New("expected a string argument")
			}
			if i := strings.Index(str, "#"); i > -1 {
				return false, nil
			}
			return true, nil
		},
		"baseuri": func(args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, errors.New("expected a string argument")
			}
			return path.Dir(str), nil
		},
		"strlen": func(args ...interface{}) (interface{}, error) {
			length := len(args[0].(string))
			return (float64)(length), nil
		},
		"epoch_to_date": func(args ...interface{}) (interface{}, error) {
			return time.Unix(int64(args[0].(float64)), 0), nil
		},
		"traverse_struct": func(args ...interface{}) (interface{}, error) {
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
		},
		"map_health_value": func(args ...interface{}) (interface{}, error) {
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
		},
	}
}
