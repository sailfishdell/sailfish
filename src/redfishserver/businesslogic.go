package redfishserver

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/structs"
)

var _ = fmt.Println
var _ = json.Marshal

// Startup - a function that should install all of the output filters and getters
func (rh *config) Startup() (done chan struct{}) {
	//ingestStartupData(rh)

	rh.odata["/redfish/"] = map[string]interface{}{"v1": "/redfish/v1/"}

	rh.odata["/redfish/v1/"] = &ServiceRoot{
		OdataType:      "#ServiceRoot.v1_0_2.ServiceRoot",
		OdataID:        "/redfish/v1/",
		OdataContext:   "/redfish/v1/$metadata#ServiceRoot",
		RedfishVersion: "v1_0_2",
		Id:             "RootService",
		Name:           "Root Service",
		Description:    "Root Service",
	}

	done = make(chan struct{})
	return done
}

type ServiceRoot struct {
	OdataType      string `json:"@odata.type"`
	OdataContext   string `json:"@odata.context"`
	OdataID        string `json:"@odata.id"`
	RedfishVersion string
	Id             string
	Name           string
	Description    string
	Services       map[string]interface{}
	Collections    map[string]interface{}
	Links          map[string]interface{}
}

func (s *ServiceRoot) MarshalJSON() ([]byte, error) {
	fmt.Println("DEBUG!")
	m := structs.Map(s)
	delete(m, "Services")
	delete(m, "Collections")
	return json.Marshal(m)
}
