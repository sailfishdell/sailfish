package redfishserver

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"path"
    "github.com/elgs/gosplitargs"
    "strconv"
)

type Service interface {
	RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (interface{}, error)
}

// ServiceMiddleware is a chainable behavior modifier for Service.
type ServiceMiddleware func(Service) Service

type Config struct {
	logger               Logger
	pickleDir            string
	serviceV1RootJSON    interface{}
	systemCollectionJSON interface{}
}

func NewService(logger Logger, pickleDir string) Service {
	cfg := Config{logger: logger, pickleDir: pickleDir}

	loadData := func() {
		var unmarshalJSONPairs = []struct {
			global   *interface{}
			filename string
		}{
			{global: &cfg.serviceV1RootJSON, filename: "serviceV1Root.json"},
			{global: &cfg.systemCollectionJSON, filename: "systemCollection.json"},
		}
		for i := range unmarshalJSONPairs {
			fileContents, e := ioutil.ReadFile(path.Join(cfg.pickleDir, unmarshalJSONPairs[i].filename))
			if e != nil {
				panic(e)
			}

			err := json.Unmarshal(fileContents, unmarshalJSONPairs[i].global)
			if err != nil {
				panic(err)
			}
		}
	}

	loadData()

	return &cfg
}

var (
	ErrNotFound = errors.New("not found")
)

func (rh *Config) RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (output interface{}, err error) {
	//logger := RequestLogger(ctx)
	//logger.Log("msg", "HELLO WORLD: rawjson")

	switch pathTemplate {
	case "/redfish/":
		output = make(map[string]string)
		output.(map[string]string)["v1"] = "/redfish/v1/"

	case "/redfish/v1/":
		return &rh.serviceV1RootJSON, nil

	case "/redfish/v1/Systems":
		return collapseCollection(rh.systemCollectionJSON.(map[string]interface{}))

	case "/redfish/v1/Systems/{system}":
		return getCollectionMember(rh.systemCollectionJSON.(map[string]interface{}), url)

	default:
		err = ErrNotFound
	}

	return
}

// implements a simple json query modeled after 'jq' command line utility
//      .       return current 
//      .NAME   return dictionary member "NAME" from current
//      .[n]    return array element 'n'
//
// Chain them together to implement fun
//
func SimpleJQL( jsonStruct interface{}, expression string ) (current interface{}, err error){
    queryElems, err := gosplitargs.SplitArgs(expression, "\\.", false)
    current = jsonStruct
    for _, queryElem := range queryElems {
        if queryElem == "." { continue }
        // is query an array?
        if len(queryElem)>3 && string(queryElem[0])=="[" && string(queryElem[len(queryElem)-1])=="]" {
            
            // pull out requested index
            idx, err := strconv.Atoi(queryElem[1:len(queryElem)-1])
            if err != nil {
                return nil, err
            }
            if arr, ok := current.([]interface{}); ok {
                if idx >= len(arr) {
                    return nil, errors.New( "requested array elem " + queryElem + " is out of bounds")
                }
                current = arr[idx]
                continue
            }

            return nil, errors.New( queryElem + " is not an array, but trying to query like one.")
        }
        // try to type assert as map[string]interface{}
        if dict, ok := current.(map[string]interface{}); ok {
            // see if map contains the requested key
            if  value, ok := dict[queryElem]; ok {
                current = value
                continue
            }
            return nil, errors.New( queryElem + " no such element")
        } else {
            return nil, errors.New( queryElem + " is not a map[string]" )
        }
    }
    return
}

// collapse the "Members": [ {...}, {...} ] so that only @odata.id appears in the output array
func collapseCollection(inputJSON map[string]interface{}) (outputJSON interface{}, err error) {
	var output map[string]interface{}
	output = make(map[string]interface{})

	// range over input, copying to output
	for k, v := range inputJSON {
		// if input is "Members", filter it
		if k == "Members" {
			// make new members array
			var members []map[string]interface{}
			for _, val := range v.([]interface{}) {
				// pull out @odata.id from input and paste it into the output
				id := val.(map[string]interface{})["@odata.id"]
				members = append(members, map[string]interface{}{"@odata.id": id})
			}
			output["Members"] = members
		} else {
			output[k] = v
		}
	}

	outputJSON = &output
	return
}

// collapse the "Members": [ {...}, {...} ] so that only @odata.id appears in the output array
func getCollectionMember(inputJSON map[string]interface{}, filter string) (interface{}, error) {
	// range over input, copying to output
	members := inputJSON["Members"]
	for _, v := range members.([]interface{}) {
		id := v.(map[string]interface{})["@odata.id"].(string)
		if id == filter {
			return v, nil
		}
	}

	return nil, ErrNotFound
}
