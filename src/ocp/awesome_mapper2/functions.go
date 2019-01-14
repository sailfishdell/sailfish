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
	"sort"

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

func CompareURLStrings (strA, strB string) bool {
	a, err := strconv.Atoi(path.Base(strA))
	if err != nil { a = 0 }
	b, err := strconv.Atoi(path.Base(strB))
	if err != nil { b = 0 }
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
			} else if CompareURLStrings (str, vStr[0]) {
				vStr = append([]string{str}, vStr...)
			} else if CompareURLStrings (vStr[len(vStr)-1], str) {
				vStr = append(vStr, str)
			} else {
				index := sort.Search(len(vStr), func(i int) bool { return CompareURLStrings (str, vStr[i]) })
				vStr = append(vStr, "")
				copy(vStr[index+1:],vStr[index:])
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
	AddFunction("get_input_voltagetype", func(args ...interface{}) (interface{}, error) {
		switch t := args[0].(float64);t {
			case 0:
				t1 := int(args[1].(float64))
				if t1 >= 100 && t1 <=127 {
					return "AC120V",nil
				}else if t1 >= 200 && t1 <=240 {
										return "AC240V",nil
								}else if t1 == 277 {
					return "AC277V",nil
				}else{
					return "Unknown",nil
				}
			case 1:
				t1 := int(args[1].(float64))
				if t1 == -48 {
										return "DCNeg48V",nil
								}else if t1 == 380 {
										return "DC380V",nil
								}else{
										return "Unknown",nil
								}
			default: return "Unknown",nil
		}
	})
	AddFunction("get_ps_state", func(args ...interface{}) (interface{}, error) {
		t := int(args[0].(float64))
		if 64 & t == 64 {
			return "Disabled",nil
		}else if 32 & t == 32 {
			return "UnavailableOffline",nil
		}else if 16 & t == 16 {
						return "UnavailableOffline",nil
				}else if 8 & t == 8 {
						return "UnavailableOffline",nil
				}else if 2 & t == 2 {
						return "Disabled",nil
				}else if 1 & t == 1 {
						return "Enabled",nil
				}else{
			return nil,nil
		}
	})
	AddFunction("get_ac_dc_value", func(args ...interface{}) (interface{}, error) {
		switch t := args[0].(float64);t {
				case 0: return "AC",nil
				case 1: return "DC",nil
				default: return "Unknown",nil
				}
	})
	AddFunction("zero_to_null", func(args ...interface{}) (interface{}, error) {
		if args[0] == 0 {
			return nil, nil
		}
		return args[0], nil
	})
	AddFunction("zero_or_value",func(args ...interface{}) (interface{}, error) {
		switch t := args[0].(float64);t {
			default:
				if t < 0 {
					return 0, nil
				}else {
					return t, nil
				}
		}
	})
	AddFunction("get_hotpluggable_value",func(args ...interface{}) (interface{}, error) {
		switch t := args[0].(float64);t {
		case 0: return false,nil
		case 1: return true,nil
		default: return nil,nil
		}
	})
	AddFunction("null_lt_zero",func(args ...interface{}) (interface{}, error) {
		if args[0] == 0 {
						return nil, nil
				}
		switch t := args[0].(float64);t {
		default:
			if t < 0 {
				return nil, nil
			}else{
				return t, nil
			}
		}
	})
	AddFunction("map_physical_context", func(args ...interface{}) (interface{}, error) { //todo: turn into hash
				switch t := args[0].(float64); t {
				case 3: return "CPU", nil
				case 4: return "StorageDevice", nil
				case 6: return "ComputeBay", nil
				case 7: return "SystemBoard", nil
		case 8: return "Memory", nil
		case 9: return "CPU", nil
		case 10: return "PowerSupply", nil
		case 12: return "Front", nil
		case 13: return "Back", nil
		case 14: return "PowerSupply", nil
		case 18: return "CPU", nil
		case 19: return "PowerSupply", nil
		case 20: return "VoltageRegulator", nil
		case 21: return "PowerSupply", nil
		case 23: return "Chassis", nil
		case 24: return "Chassis", nil
		case 25: return "ComputeBay", nil
		case 29: return "Fan", nil
		case 30: return "Fan", nil
		case 32: return "Memory", nil
		case 41: return "ComputeBay", nil
		case 42: return "NetworkDevice", nil
		case 43: return "NetworkDevice", nil
		case 46: return "Chassis", nil
				default:
						return "Chassis", errors.New("Invalid object status")
				}
		})
	AddFunction("empty_to_null", func(args ...interface{}) (interface{}, error) {
		if args[0] == "" {
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
	AddFunction("encryptn_ability", func(args ...interface{}) (interface{}, error) {
		var attributes int64 = int64(args[0].(float64))
		if attributes&0x04 == 0x04 {
			return "SelfEncryptingDrive", nil
		} else {
			return "None", nil
		}
	})
	AddFunction("encryptn_status", func(args ...interface{}) (interface{}, error) {
		var security int64 = int64(args[0].(float64))
		if security&0x01 == 0x01 {
			return "Unlocked", nil
		} else if security&0x02 == 0x02 {
			return "Locked", nil
		} else if security&0x04 == 0x04 {
			return "Foreign", nil
		} else {
			return "Unencrypted", nil
		}
	})
	AddFunction("fail_predicted", func(args ...interface{}) (interface{}, error) {
		var attributes int64 = int64(args[0].(float64))
		var objattributes int64 = int64(args[1].(float64))

		if attributes&0x01 == 0x01 && objattributes&01 == 0x01 {
			return true, nil
		} else {
			return false, nil
		}
	})

	AddFunction("hotspare", func(args ...interface{}) (interface{}, error) {
		var hotspare int8 = int8(args[0].(float64))
		if hotspare&0x01 == 0x01 {
			return "Dedicated", nil
		} else if hotspare&0x02 == 0x02 {
			return "Global", nil
		} else {
			return "None", nil
		}
	})
	AddFunction("durable_name", func(args ...interface{}) (interface{}, error) {
		wwn, _ := strconv.Atoi(args[0].(string))
		if wwn > 0x00 {
			identif := fmt.Sprintf("%X", wwn)
			return identif, nil
		} else {
			return nil, nil
		}
	})
	AddFunction("durable_format", func(args ...interface{}) (interface{}, error) {
		wwn, _ := strconv.Atoi(args[0].(string))
		if wwn > 0x00 {
			return "NAA", nil
		} else {
			return nil, nil
		}
	})
	AddFunction("identifier_gen", func(args ...interface{}) (interface{}, error) {
		fmt.Printf("in identifier_gen()\n")
		wwnStr := args[0].(string)
		fmt.Printf("in identifier_gen() -- %s\n", wwnStr)
		wwn, _ := strconv.ParseInt(wwnStr, 16, 64)
		fmt.Printf("wwn in identifier_gen() -- %d\n", wwn)
		if wwn > 0x00 {
			dur_name := fmt.Sprintf("%X", wwn)
			dur_format := "NAA"
			fmt.Printf("Returning identifier: %s\n", []map[string]string{map[string]string{"DurableName": dur_name, "DurableNameFormat": dur_format}})
			return []map[string]string{map[string]string{"DurableName": dur_name, "DurableNameFormat": dur_format}}, nil
		} else {
			fmt.Printf("wwn is nil, returning nil")
			return nil, nil
		}
	})

	AddFunction("encryptionstatus", func(args ...interface{}) (interface{}, error) {
		fmt.Printf("in encryption types()\n")
		var attrib uint32 = uint32(args[0].(float64))
		if attrib&0x01 == 0x01 {
			return true, nil
		} else {
			return false, nil
		}
	})

	AddFunction("encryptionmode", func(args ...interface{}) (interface{}, error) { //todo: turn into hash
		switch t := args[0].(float64); t {
		case 0:
			return "None", nil
		case 1:
			return "LocalKeyManagement", nil
		case 2:
			return "DellKeyManagement", nil
		case 3:
			return "PendingDellKeyManagementCapable", nil
		default:
			return nil, errors.New("Invalid object status")
		}
	})
	AddFunction("volumetype", func(args ...interface{}) (interface{}, error) {
		fmt.Printf("in volume types()\n")
		var raidlevel uint32 = uint32(args[0].(float64))
		if raidlevel&0x01 == 0x01 {
			return "RawDevice", nil
		} else if raidlevel&0x00000002 == 0x00000002 {
			return "NonRedundant", nil
		} else if raidlevel&0x00000004 == 0x00000004 {
			return "Mirrored", nil
		} else if raidlevel&0x00000040 == 0x00000040 {
			return "StripedWithParity", nil
		} else if raidlevel&0x00000800 == 0x00000800 {
			return "SpannedMirrors", nil
		} else if raidlevel&0x00002000 == 0x00002000 || raidlevel&0x00004000 == 0x00004000 {
			return "SpannedStripesWithParity", nil
		} else {
			return nil, nil
		}
	})

	AddFunction("encryptiontypes", func(args ...interface{}) (interface{}, error) {
		var vStr []string
		vStr = append(vStr, "NativeDriveEncryption")
		return vStr, nil
	})

	AddFunction("mediatype", func(args ...interface{}) (interface{}, error) {
		var attrib uint32 = uint32(args[0].(float64))
		if attrib&0x02 == 0x02 {
			return "SSD", nil
		} else {
			return "HDD", nil
		}
	})

	AddFunction("powerstate", func(args ...interface{}) (interface{}, error) {
		var powerstate uint32 = uint32(args[0].(float64))
		if powerstate == 3 {
			return "On", nil
		} else if powerstate == 1 {
			return "Off", nil
		} else if powerstate == 2 {
			return "PoweringOn", nil
		} else if powerstate == 4 {
			return "PoweringOff", nil
		} else {
			return nil, nil
		}
	})

	AddFunction("driveformfactor", func(args ...interface{}) (interface{}, error) {
		var driveFF uint32 = uint32(args[0].(float64))
		if driveFF == 0 {
			return "Unknown", nil
		} else if driveFF == 1 {
			return "1.5Inch", nil
		} else if driveFF == 2 {
			return "2.5Inch", nil
		} else if driveFF == 3 {
			return "3.5Inch", nil
		} else {
			return nil, nil
		}
	})

	AddFunction("smartalert", func(args ...interface{}) (interface{}, error) {
		var smartAlt uint32 = uint32(args[0].(float64))
		if smartAlt&0x00000001 == 0x00000001 {
			return "SmartAlertPresent", nil
		} else {
			return "SmartAlertAbsent", nil
		}
	})

	AddFunction("raidstatus", func(args ...interface{}) (interface{}, error) {
		var vStr string
		var raidStatus uint32 = uint32(args[0].(float64))
		var g_raidStatus = []string{"Unknown", "Ready", "Online", "Foreign", "Offline", "Blocked", "Failed", "Degraded", "NonRAID", "Missing"}
		switch raidStatus {
		case 1:
			return g_raidStatus[1], nil
		case 2:
			return g_raidStatus[2], nil
		case 3:
			return g_raidStatus[3], nil
		case 4:
			return g_raidStatus[4], nil
		case 5:
			return g_raidStatus[5], nil
		case 6:
			return g_raidStatus[6], nil
		case 7:
			return g_raidStatus[7], nil
		case 8:
			return g_raidStatus[8], nil
		case 9:
			return g_raidStatus[9], nil
		default:
			return g_raidStatus[0], nil
		}
		return vStr, nil
	})

	AddFunction("supporteddeviceprotocols", func(args ...interface{}) (interface{}, error) {
		var vStr []string
		var deviceprotocols uint32 = uint32(args[0].(float64))

		if deviceprotocols&0x00000001 == 0x00000001 {
			vStr = append(vStr, "SCSI")
		}
		if deviceprotocols&0x00000002 == 0x00000002 {
			vStr = append(vStr, "PATA")
		}
		if deviceprotocols&0x00000004 == 0x00000004 {
			vStr = append(vStr, "FIBRE")
		}
		if deviceprotocols&0x00000008 == 0x00000008 {
			vStr = append(vStr, "USB")
		}
		if deviceprotocols&0x00000010 == 0x00000010 {
			vStr = append(vStr, "SATA")
		}
		if deviceprotocols&0x00000020 == 0x00000020 {
			vStr = append(vStr, "SAS")
		}
		if deviceprotocols&0x00000040 == 0x00000040 {
			vStr = append(vStr, "PCIE")
		}
		if deviceprotocols&0x00000100 == 0x00000100 {
			vStr = append(vStr, "NVMe")
		}
		return vStr, nil
	})

	AddFunction("deviceprotocols", func(args ...interface{}) (interface{}, error) {
		var vStr string
		var deviceprotocols uint32 = uint32(args[0].(float64))

		if deviceprotocols&0x00000001 == 0x00000001 {
			vStr = "SCSI"
		}
		if deviceprotocols&0x00000002 == 0x00000002 {
			vStr = "PATA"
		}
		if deviceprotocols&0x00000004 == 0x00000004 {
			vStr = "FIBRE"
		}
		if deviceprotocols&0x00000008 == 0x00000008 {
			vStr = "USB"
		}
		if deviceprotocols&0x00000010 == 0x00000010 {
			vStr = "SATA"
		}
		if deviceprotocols&0x00000020 == 0x00000020 {
			vStr = "SAS"
		}
		if deviceprotocols&0x00000040 == 0x00000040 {
			vStr = "PCIE"
		}
		if deviceprotocols&0x00000100 == 0x00000100 {
			vStr = "NVMe"
		}
		return vStr, nil
	})

	AddFunction("cachecade", func(args ...interface{}) (interface{}, error) {
		var vStr string
		var attributes uint32 = uint32(args[0].(float64))

		if attributes&0x00000080 == 0x00000080 {
			vStr = "CachecadeVD"
		} else {
			vStr = "NonCachecadeVD"
		}
		return vStr, nil
	})

	AddFunction("diskcachepolicy", func(args ...interface{}) (interface{}, error) {
		var vStr string
		var diskCachePolicy uint32 = uint32(args[0].(float64))

		if diskCachePolicy&0x00000100 == 0x00000100 {
			vStr = "Default"
		}
		if diskCachePolicy&0x00000200 == 0x00000200 {
			vStr = "Enabled"
		}
		if diskCachePolicy&0x00000400 == 0x00000400 {
			vStr = "Disabled"
		}
		return vStr, nil
	})

	AddFunction("readcachepolicy", func(args ...interface{}) (interface{}, error) {
		var vStr string
		var readCachePolicy uint32 = uint32(args[0].(float64))

		if readCachePolicy&0x00000010 == 0x00000010 {
			vStr = "NoReadAhead"
		}
		if readCachePolicy&0x00000020 == 0x00000020 {
			vStr = "ReadAhead"
		}
		if readCachePolicy&0x00000040 == 0x00000040 {
			vStr = "AdaptiveReadAhead"
		}
		return vStr, nil
	})

	AddFunction("writecachepolicy", func(args ...interface{}) (interface{}, error) {
		var vStr string
		var writeCachePolicy uint32 = uint32(args[0].(float64))

		if writeCachePolicy&0x000001 == 0x000001 {
			vStr = "WriteThrough"
		}
		if writeCachePolicy&0x000002 == 0x000002 {
			vStr = "WriteBack"
		}
		if writeCachePolicy&0x000004 == 0x000004 {
			vStr = "WriteBackForce"
		}
		return vStr, nil
	})

	AddFunction("lockstatus", func(args ...interface{}) (interface{}, error) {
		var vStr string
		var attributes uint32 = uint32(args[0].(float64))

		if attributes&0x00000001 == 0x00000001 {
			vStr = "Locked"
		} else {
			vStr = "UnLocked"
		}
		return vStr, nil
	})

	AddFunction("cachecadecap", func(args ...interface{}) (interface{}, error) {
		var vStr string
		var attributes uint32 = uint32(args[0].(float64))

		if attributes&0x00002000 == 0x00002000 {
			vStr = "Supported"
		} else {
			vStr = "NotSupported"
		}
		return vStr, nil
	})

	AddFunction("slottype", func(args ...interface{}) (interface{}, error) {
		var vStr string
		var slottype uint32 = uint32(args[0].(float64))
		var g_sSlotType = []string{"Unknown", "PCI Express x8", "PCI Express Gen3", "PCI Express Gen3 x1",
			"PCI Express Gen3 x2", "PCI Express Gen3 x4", "PCI Express Gen3 x8", "PCI Express Gen3 x16", "PCI Express x16",
			"PCI Express", "PCI Express x1", "PCI Express x2", "PCI Express x4", "PCI Express Gen2 x16", "PCI Express Gen2"}
		switch slottype {
		case 0xA9:
			return g_sSlotType[1], nil
		case 0xB1:
			return g_sSlotType[2], nil
		case 0xB2:
			return g_sSlotType[3], nil
		case 0xB3:
			return g_sSlotType[4], nil
		case 0xB4:
			return g_sSlotType[5], nil
		case 0xB5:
			return g_sSlotType[6], nil
		case 0xB6:
			return g_sSlotType[7], nil
		case 0xAA:
			return g_sSlotType[8], nil
		case 0xA5:
			return g_sSlotType[9], nil
		case 0xA6:
			return g_sSlotType[10], nil
		case 0xA7:
			return g_sSlotType[11], nil
		case 0xA8:
			return g_sSlotType[12], nil
		case 0xB0:
			return g_sSlotType[13], nil
		case 0xAB:
			return g_sSlotType[14], nil
		default:
			return g_sSlotType[0], nil
		}
		return vStr, nil
	})

	AddFunction("encryptionncap", func(args ...interface{}) (interface{}, error) {
		var attributes int64 = int64(args[0].(float64))
		if attributes&0x00000080 == 0x00000080 {
			return "LocalKeyManagementCapable", nil
		}
		if attributes&0x00000400 == 0x00000400 {
			return "LocalKeyManagementAndDellKeyManagementCapable", nil
		}
		return nil, nil
	})

	AddFunction("pcislot", func(args ...interface{}) (interface{}, error) {
		var embedded int64 = int64(args[0].(float64))
		var slot int64 = int64(args[1].(float64))
		fmt.Printf("embedded -- %d\n", embedded)
		fmt.Printf("slot -- %d\n", slot)
		if embedded != 0 {
			return int(slot), nil
		}
		return nil, nil
	})

	AddFunction("patrolstate", func(args ...interface{}) (interface{}, error) {
		var patrolState int64 = int64(args[0].(float64))
		if patrolState&0x10 == 0x10 {
			return "Stopped", nil
		}
		if patrolState&0x20 == 0x20 {
			return "Running", nil
		}
		return nil, nil
	})

	AddFunction("rollupstatus", func(args ...interface{}) (interface{}, error) {
		var rollupStatus int64 = int64(args[0].(float64))
		fmt.Printf("Input for rollupStatus -- %d\n", rollupStatus)
		if rollupStatus == 0x1 {
			return "Unknown", nil
		}
		if rollupStatus == 0x2 {
			return "Ok", nil
		}
		if rollupStatus == 0x3 {
			return "Error", nil
		}
		if rollupStatus == 0x4 {
			return "Degraded", nil
		}
		return nil, nil
	})

	AddFunction("capablespeeds", func(args ...interface{}) (interface{}, error) {
		var speed float32
		fmt.Printf("cpbspeeds -- %d\n", int(args[0].(float64)))
		switch cpbspeeds := int(args[0].(float64)); cpbspeeds {
		case 15:
			speed = 12
		case 7, 4:
			speed = 3
		case 3:
			speed = 1
		case 1:
			speed = 1.5
		}
		fmt.Printf("speed -- %f\n", speed)
		return speed, nil
	})

	AddFunction("negospeeds", func(args ...interface{}) (interface{}, error) {
		var speed float32
		fmt.Printf("negospeeds -- %d\n", int(args[0].(float64)))
		switch negospeeds := int(args[0].(float64)); negospeeds {
		case 64:
			speed = 64
		case 32:
			speed = 32
		case 16:
			speed = 16
		case 8:
			speed = 12
		case 7, 4:
			speed = 6
		case 2:
			speed = 3
		case 1:
			speed = 1.5
		}
		fmt.Printf("speed -- %f\n", speed)
		return speed, nil
	})

	AddFunction("securitystatus", func(args ...interface{}) (interface{}, error) {
		var secStatus int64 = int64(args[0].(float64))
		if secStatus&0x00000100 == 0x00000100 {
			return "SecurityKeyAssigned", nil
		} else if secStatus&0x00000080 == 0x00000080 {
			return "EncryptionCapable", nil
		} else {
			return "EncryptionNotCapable", nil
		}
		return nil, nil
	})

	AddFunction("slicedvdcap", func(args ...interface{}) (interface{}, error) {
		var slicedVDCap int64 = int64(args[0].(float64))
		if slicedVDCap&0x00040000 == 0x00040000 {
			return "Supported", nil
		} else {
			return "NotSupported", nil
		}
		return nil, nil
	})

	AddFunction("controllerprotocols", func(args ...interface{}) (interface{}, error) {
		var vStr []string
		vStr = append(vStr, "PCIe")
		return vStr, nil
	})
}
