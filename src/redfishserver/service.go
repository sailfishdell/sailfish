package redfishserver

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"path"
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
