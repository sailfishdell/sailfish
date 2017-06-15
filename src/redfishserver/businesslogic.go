package redfishserver

import (
    "encoding/json"
    "fmt"
)

var _ = fmt.Println

// Startup - a function that should install all of the output filters and getters
func (rh *config) Startup() (done chan struct{}) {
	//ingestStartupData(rh)

    rh.odata = make( map[string]interface{} )
    rh.odata["/redfish/"] = &struct{ V1 string `json:"v1"` }{ V1: "/redfish/v1/" }

    odataservices := make(OdataServices)
    odataservices["mars"] = "rocky planet"
    rh.odata["/redfish/v1/"] = &ServiceRoot{
        StaticServiceRoot: &StaticServiceRoot{
        OdataBase: &OdataBase{
            Type: "#ServiceRoot.v1_0_2.ServiceRoot",
            ID:   "/redfish/v1/",
            Context: "/redfish/v1/$metadata#ServiceRoot",
        },
        RedfishVersion: "v1_0_2",
        Id: "RootService",
        Name: "Root Service",
        Description: "Root Service",
        },
        OdataServices: &odataservices,
    }

	done = make(chan struct{})
	return done
}


type ServiceRoot struct {
    *StaticServiceRoot
    *OdataServices
}

func (s *ServiceRoot) MarshalJSON() ([]byte, error){
    fmt.Println("TEST ServiceRoot")
    return []byte(`"foo"`), nil
}

type OdataServices map[string]interface{}
func (s *OdataServices) MarshalJSON() ([]byte, error){
    fmt.Println("TEST 222")
    return []byte(`"foo"`), nil
}

type omit *struct{}

func (s *StaticServiceRoot) MarshalJSON() ([]byte, error) {
    type Alias StaticServiceRoot
    fmt.Println("TEST StaticServiceRoot")
    return  json.Marshal(
        &struct {
            OdataServices omit `json:"s,omitempty"`
            *Alias
        }{
            Alias: (*Alias)(s),
        })
}

type StaticServiceRoot struct {
    RedfishVersion string
    Id      string
    Name    string
    Description    string
    *OdataBase
}

func (s *OdataBase) MarshalJSON() ([]byte, error){
    fmt.Println("TEST OdataBase")
    return []byte(`"foo"`), nil
}

type OdataBase struct {
	Type        string `json:"@odata.type"`
	Context     string `json:"@odata.context"`
	ID      string `json:"@odata.id"`
}






