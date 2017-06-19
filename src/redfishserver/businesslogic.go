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
	serviceRoot := rh.odata.NewServiceRoot()
	systems := rh.odata.AddCollection(
		serviceRoot,
		"Systems",
		"System Collection",
		"#ComputerSystemCollection.ComputerSystemCollection",
		"/redfish/v1/Systems",
		"/redfish/v1/$metadata#Systems",
	)
    
    rh.odata.AddSystem(systems)

	done = make(chan struct{})
	return done
}

func (odata OdataTree) NewServiceRoot() OdataTreeInt {
	ret := &ServiceRoot{
		RedfishVersion: "1.0.2",
		Id:             "RootService",
		Name:           "Root Service",
		Description:    "The root service",
	}

	ret.OdataBase = NewOdataBase(
		"/redfish/v1/",
		"/redfish/v1/$metadata#ServiceRoot",
		"#ServiceRoot.v1_0_2.ServiceRoot",
		&odata,
		ret,
	)

	return ret
}

func (odata OdataTree) AddCollection(sr OdataTreeInt, nodeName, name, otype, id, ocontext string) *Collection {
	ret := &Collection{
		Name:    name,
		Members: []map[string]interface{}{},
	}

	ret.OdataBase = NewOdataBase(
		id,
		ocontext,
		otype,
		&odata,
		ret,
	)

	sr.AddNode(nodeName, ret)
	return ret
}

func (odata OdataTree) AddSystem(c *Collection) {
    id := "/redfish/v1/Systems/TEST"
	ret := &System{
		Name:    "TEST",
	}

	ret.OdataBase = NewOdataBase(
		id,
		"/redfish/v1/$metadata#ComputerSystem.ComputerSystem",
		"#ComputerSystem.v1_1_0.ComputerSystem",
		&odata,
		ret,
	)

    c.Members = append(c.Members, map[string]interface{}{"@odata.id": id})
}
