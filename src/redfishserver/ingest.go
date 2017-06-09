package redfishserver

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"strings"

	"fmt"
)

func ingestStartupData(cfg *Config) {
	cfg.odata = make(map[string]interface{})

	err := ingest(filenameFromID_SPMF, "mockups/DSP2043-server/", "index.json", "/redfish/v1/", cfg.odata)
	//err := ingest( filenameFromID_ec, "ec", "_redfish_v1.json", "/redfish/v1", cfg.odata )
	if err != nil {
		panic(err)
	}
}

func getOdata(pathname string, filename string, store interface{}) (interface{}, error) {
	fileContents, err := ioutil.ReadFile(path.Join(pathname, filename))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(fileContents, store)
	if err != nil {
		return nil, err
	}

	return store, nil
}

func ingest(filenameFromID func(string) string, basepath string, filename string, odataid string, odata map[string]interface{}) error {
	fmt.Printf("Ingesting file(%s) for @odata.id = %s\n", path.Join(basepath, filename), odataid)

	var submap map[string]interface{}
	_, err := getOdata(basepath, filename, &submap)
	if err != nil {
		fmt.Printf("  that failed... %s\n", err)
		return err
	}
	odata[odataid] = submap
	subids := getNestedOdataIds(odata[odataid], true)
	for _, id := range subids {
		// prevent loops, only import if not already imported
		if _, ok := odata[id]; !ok {
			ingest(filenameFromID, basepath, filenameFromID(id), id, odata)
		}
	}
	return nil
}

func filenameFromID_SPMF(id string) string {
	id = strings.SplitN(id, "#", 2)[0]
	id = strings.Replace(id, "/redfish/v1/", "", -1)
	// id = strings.Replace(id, "/", "_", -1)
	return id + "/index.json"
}

func filenameFromID_ec(id string) string {
	id = strings.Replace(id, "|", "X", -1)
	id = strings.Replace(id, "/", "_", -1)
	return id + ".json"
}

func getNestedOdataIds(inputJSON interface{}, allowonce bool) (output []string) {
	var nestedIds []string

	// range over input, copying to output
	switch nested := inputJSON.(type) {
	case map[string]interface{}:
		if _, ok := nested["@odata.id"]; ok && (!allowonce) {
			nestedIds = append(nestedIds, nested["@odata.id"].(string))
		}

		for _, v := range nested {
			nestedIds = append(nestedIds, getNestedOdataIds(v, false)...)
		}

	case []interface{}:
		for _, mem := range nested {
			nestedIds = append(nestedIds, getNestedOdataIds(mem, false)...)
		}
	}
	return nestedIds
}
