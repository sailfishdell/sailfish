package redfishserver

import (
	"encoding/json"
	"io/ioutil"
	"path"
)

func ingestStartupData(cfg *Config) {
	// for backwards compat stuff until we switch to unified
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
	/*
	   startpath := "../ec/"
	   startingpoint := "_redfish_v1.json"
	       fileContents, e := ioutil.ReadFile(path.Join(cfg.pickleDir, unmarshalJSONPairs[i].filename))
	       if e != nil {
	           panic(e)
	       }

	       err := json.Unmarshal(fileContents, unmarshalJSONPairs[i].global)
	       if err != nil {
	           panic(err)
	       }
	*/
}
