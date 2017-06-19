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

	rh.odata.AddServiceRoot()

	done = make(chan struct{})
	return done
}

func (odata OdataTree) AddServiceRoot() OdataTreeInt {
	ret := &ServiceRoot{
		RedfishVersion: "1.0.2",
		Id:             "RootService",
		Name:           "Root Service",
		Description:    "The root service",
		OdataBase: &OdataBase{
			OdataType:    "#ServiceRoot.v1_0_2.ServiceRoot",
			OdataID:      "/redfish/v1/",
			OdataContext: "/redfish/v1/$metadata#ServiceRoot",
		},
	}
	ret.OdataBase.wrapper = ret
	odata["/redfish/v1/"] = ret
	return ret
}
