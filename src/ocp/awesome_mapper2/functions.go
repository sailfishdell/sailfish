package awesome_mapper

import (
	"errors"
	"reflect"
	"strconv"
	"time"

	"github.com/Knetic/govaluate"
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
