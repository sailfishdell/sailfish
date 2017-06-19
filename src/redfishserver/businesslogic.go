package redfishserver

import (
	"fmt"
)

var _ = fmt.Println

// Startup - a function that should install all of the output filters and getters
func (rh *config) Startup() (done chan struct{}) {
	//ingestStartupData(rh)

	// add the top level redfish version pointer. This is always completely static
	rh.odata["/redfish/"] = map[string]interface{}{"v1": "/redfish/v1/"}

	// Manually add the redfish structure, since the structure itself is always going to be completely static
//	serviceRoot := rh.odata.AddServiceRoot()
//	rh.odata.AddSystemsCollection(serviceRoot)

    rh.odata.AddSR2()

	done = make(chan struct{})
	return done
}


func (odata OdataTree) AddSR2() (OdataTreeInt) {
	ret := &SR2{
        RedfishVersion: "HAPPY",
		OdataBase: &OdataBase{
            OdataType:      "#ServiceRoot.v1_0_2.ServiceRoot",
		    OdataID:        "/redfish/v1/",
		    OdataContext:   "/redfish/v1/$metadata#ServiceRoot",
            },
	}
    ret.OdataBase.wrapper = ret
	odata["/redfish/v1/"] = ret
	return ret
}



func (odata OdataTree) AddServiceRoot() (sr *ServiceRoot) {
	sr = &ServiceRoot{
		OdataType:      "#ServiceRoot.v1_0_2.ServiceRoot",
		OdataID:        "/redfish/v1/",
		OdataContext:   "/redfish/v1/$metadata#ServiceRoot",
		RedfishVersion: "v1_0_2",
		Id:             "RootService",
		Name:           "Root Service",
		Description:    "Root Service",
		OdataTree:      &odata,
		Collections:    map[string]interface{}{},
		Links:          map[string]interface{}{},
		Services:       map[string]interface{}{},
	}
	odata["/redfish/v1/"] = sr
	return
}

func (odata OdataTree) AddSystemsCollection(sr *ServiceRoot) (systems *OdataCollection) {
	systems = &OdataCollection{
		OdataType:    "#ComputerSystemCollection.ComputerSystemCollection",
		OdataContext: "/redfish/v1/$metadata#Systems",
		BaseOdataID:  BaseOdataID{OdataID: "/redfish/v1/Systems"},
		Name:         "Computer System Collection",
		Description:  "Computer System Collection",
		Members:      []interface{}{},
		Oem:          map[string]interface{}{},
	}
	sr.AddCollection("Systems", systems)
	return
}
