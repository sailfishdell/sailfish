package awesome_mapper

import (
	"errors"
	"fmt"
	"path"
	"reflect"
	"strconv"
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
				return nil, errors.New("Cant parse non-string")
			}
		},
		"removefromset": func(args ...interface{}) (interface{}, error) {
			model, ok := args[0].(*model.Model)
			if !ok {
				return nil, errors.New("Need model as first arg!")
			}
			property, ok := args[1].(string)
			if !ok {
				return nil, errors.New("Need property name as second arg!")
			}
			str, ok := args[2].(string)
			if !ok {
				return nil, errors.New("Need new value as third arg!")
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

			for i, _ := range vStr {
				if vStr[i] == str {
					ret := vStr[:i]
					if i+1 < len(vStr) {
						ret = append(ret, vStr[i+1:]...)
					}
					fmt.Printf("REDUCED SET: minus:(%s) = %s\n", str, ret)
					model.UpdateProperty(property, ret)
					break
				}
			}
			return nil, errors.New("unimplemented")
		},
		"addtoset": func(args ...interface{}) (interface{}, error) {
			model, ok := args[0].(*model.Model)
			if !ok {
				return nil, errors.New("Need model as first arg!")
			}
			property, ok := args[1].(string)
			if !ok {
				return nil, errors.New("Need property name as second arg!")
			}
			str, ok := args[2].(string)
			if !ok {
				return nil, errors.New("Need new value as third arg!")
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
			for i, _ := range vStr {
				if vStr[i] == str {
					found = true
				}
			}
			if !found {
				vStr = append(vStr, str)
				model.UpdateProperty(property, vStr)
				fmt.Printf("UPDATED SET: %s\n", vStr)
			}
			return nil, errors.New("unimplemented")
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
