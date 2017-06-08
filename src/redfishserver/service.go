package redfishserver

import (
	"context"
	"errors"
	"github.com/elgs/gosplitargs"
	"strconv"
)

type Service interface {
	RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (interface{}, error)
	Startup() chan struct{}
}

// ServiceMiddleware is a chainable behavior modifier for Service.
type ServiceMiddleware func(Service) Service

type Config struct {
	logger               Logger
	pickleDir            string
	serviceV1RootJSON    interface{}
	systemCollectionJSON interface{}

    // lets put all the data here, eventually...
    odata  map[string]interface{}
}

var (
	ErrNotFound = errors.New("not found")
)

func NewService(logger Logger, pickleDir string) Service {
	cfg := Config{logger: logger, pickleDir: pickleDir}

    ingestStartupData(&cfg)
	return &cfg
}

func (rh *Config) RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (output interface{}, err error) {

	switch pathTemplate {
	case "/redfish/":
		output = make(map[string]string)
		output.(map[string]string)["v1"] = "/redfish/v1/"

	case "/redfish/v1/":
		return &rh.serviceV1RootJSON, nil

	case "/redfish/v1/Systems":
		return elideNestedOdataRefs(rh.systemCollectionJSON, true), nil

	case "/redfish/v1/Systems/{system}":
		system, _ := getCollectionMember(rh.systemCollectionJSON, url)
		return elideNestedOdataRefs(system, true), nil

	case "/redfish/v1/Systems/{system}/BIOS":
		system, _ := getCollectionMember(rh.systemCollectionJSON, "/redfish/v1/Systems/"+args["system"])
		bios, _ := SimpleJQL(system, "Bios")
		output := elideNestedOdataRefs(bios, true)
		return output, nil

	case "/redfish/v1/Systems/{system}/BIOS/Settings":
		system, _ := getCollectionMember(rh.systemCollectionJSON, "/redfish/v1/Systems/"+args["system"])
		bios, _ := SimpleJQL(system, "Bios.'@Redfish.Settings'.SettingsObject")
		output := elideNestedOdataRefs(bios, true)
		return output, nil

    case "/redfish/v1/Systems/{system}/EthernetInterfaces":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/EthernetInterfaces/{interfaceid}":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/EthernetInterfaces/{interfaceid}/VLANs":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/EthernetInterfaces/{interfaceid}/VLANs/{vlanid}":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/LogServices":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/LogServices/{logid}":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/LogServices/Log1/Entries":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/LogServices/Log1/Entries/{logentry}":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/Memory":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/Memory/{dimmid}":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/Processors":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/Processors/{cpuid}":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/SimpleStorage":
		err = ErrNotFound
    case "/redfish/v1/Systems/{system}/SimpleStorage/{storageid}":
		err = ErrNotFound

	default:
		err = ErrNotFound
	}

	return
}

// implements a simple json query modeled after 'jq' command line utility (reduced syntax!)
//      .       return current
//      .NAME   return dictionary member "NAME" from current
//      .[n]    return array element 'n'
//
// Chain them together to implement fun
//
func SimpleJQL(jsonStruct interface{}, expression string) (current interface{}, err error) {
	queryElems, err := gosplitargs.SplitArgs(expression, "\\.", false)
	current = jsonStruct
	for _, queryElem := range queryElems {
		if queryElem == "." {
			continue
		}
		// is query an array?
		if len(queryElem) >= 3 && string(queryElem[0]) == "[" && string(queryElem[len(queryElem)-1]) == "]" {
			// pull out requested index
			idx, err := strconv.Atoi(queryElem[1 : len(queryElem)-1])
			if err != nil {
				return nil, err
			}
			if arr, ok := current.([]interface{}); ok {
				if idx >= len(arr) {
					return nil, errors.New("requested array elem " + queryElem + " is out of bounds")
				}
				current = arr[idx]
				continue
			}

			return nil, errors.New("attempt to array index into a non-array")
		}
		// try to type assert as map[string]interface{}
		if dict, ok := current.(map[string]interface{}); ok {
			// see if map contains the requested key
			if value, ok := dict[queryElem]; ok {
				current = value
				continue
			}
			return nil, errors.New(queryElem + " no such element")
		} else {
			return nil, errors.New("current element is not a map[string] and cannot be indexed")
		}
	}
	return
}

func getCollectionMember(inputJSON interface{}, filter string) (interface{}, error) {
	// aggressively check types. bail if oddness
	if inputmap, ok := inputJSON.(map[string]interface{}); ok {
		if members, ok := inputmap["Members"]; ok {
			if membersarray, ok := members.([]interface{}); ok {
				for _, v := range membersarray {
					if submap, ok := v.(map[string]interface{}); ok {
						id := submap["@odata.id"]
						if idstr, ok := id.(string); ok {
							if idstr == filter {
								return submap, nil
							}
						}
					}
				}
			}
		}
	}
	return nil, ErrNotFound
}

//
// should be able to build a $filter thingy on top of this at some point
//
func elideNestedOdataRefs(inputJSON interface{}, allowonce bool) (output interface{}) {
	// range over input, copying to output
	switch nested := inputJSON.(type) {
	case map[string]interface{}:
		var output map[string]interface{}
		output = make(map[string]interface{})
		if _, ok := nested["@odata.id"]; ok && (!allowonce) {
			output["@odata.id"] = nested["@odata.id"]
		} else {
			for k, v := range nested {
				output[k] = elideNestedOdataRefs(v, false)
			}
		}
		return output

	case []interface{}:
		var outputArr []interface{}
		for _, mem := range nested {
			outputArr = append(outputArr, elideNestedOdataRefs(mem, false))
		}
		return outputArr

	default:
		return inputJSON
	}
}
