package redfishserver

import (
	"fmt"
)

var _ = fmt.Println

func makeFullyQualifiedV1(rh *config, path string) string {
    return rh.baseURI + "/" + rh.verURI + "/" + path
}

// Startup - a function that should install all of the output filters and getters
func (rh *config) Startup() (done chan struct{}) {
	//ingestStartupData(rh)

	// add the top level redfish version pointer. This is always completely static
	rh.odata[ rh.baseURI + "/" ] = map[string]interface{}{"v1": makeFullyQualifiedV1(rh, "")}

	// Manually add the redfish structure, since the structure itself is always going to be completely static
	serviceRoot := rh.NewServiceRoot(rh.odata)
	systems := rh.odata.AddCollection(
		serviceRoot,
		"Systems",
		"System Collection",
		"#ComputerSystemCollection.ComputerSystemCollection",
		makeFullyQualifiedV1(rh, "Systems"),
		makeFullyQualifiedV1(rh, "$metadata#Systems"),
	)

    rh.AddSystem(rh.odata, systems)

	done = make(chan struct{})
	return done
}

func (rh *config) NewServiceRoot(odata OdataTree) OdataTreeInt {
	ret := &ServiceRoot{
		RedfishVersion: "1.0.2",
		Id:             "RootService",
		Name:           "Root Service",
		Description:    "The root service",
	}

	ret.OdataBase = NewOdataBase(
		makeFullyQualifiedV1(rh, ""),
		makeFullyQualifiedV1(rh, "$metadata#ServiceRoot"),
		"#ServiceRoot.v1_0_2.ServiceRoot",
		&odata,
		ret,
	)

	return ret
}

