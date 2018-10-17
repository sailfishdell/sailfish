package awesome_mapper2

import (
	"errors"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"
  "io/ioutil"
  "os"

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
    "string": func(args ...interface{}) (interface{}, error) {
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
    },
    "zero_to_null": func(args ...interface{}) (interface{}, error) {
      if (args[0] == 0) {
        return nil, nil
      }
      return args[0], nil
    },
    "subsystem_health": func(args ...interface{}) (interface{}, error) {
      fqdd := strings.Split(args[0].(map[string]string)["FQDD"], "#")
      subsys := fqdd[len(fqdd)-1]
      health := args[0].(map[string]string)["Health"]
      if (health == "Absent") {
        return nil, nil
      }
      return map[string]string{"subsys":subsys, health:"health"}, nil
    },
    "read_file": func(args ...interface{}) (interface{}, error) {
      lines := ""
      file_path := args[0].(string)
      if (file_path == "NONE") {
        return nil, nil
      }
      bytes, err := ioutil.ReadFile(file_path)
      if (err != nil) {
        return nil, err
      } else {
        lines = string(bytes)
      }
      err = os.Remove(file_path)
      if (err != nil) {
        return lines, err
      }
      return lines, nil
    },
	}
}
