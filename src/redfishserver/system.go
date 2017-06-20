package redfishserver

import ()

type System struct {
	*OdataBase
	Id           string
	Name         string
	SystemType   string
	AssetTag     string
	Manufacturer string
	Model        string
	SKU          string
	SerialNumber string
	PartNumber   string
	Description  string
	UUID         string
	HostName     string
	Status       struct {
		State        string
		Health       string
		HealthRollUp string
	}
}

func (rh *config) AddSystem(odata OdataTree, c *Collection) {
    id := makeFullyQualifiedV1(rh, "Systems/TEST")
	ret := &System{
		Name:    "TEST",
	}

	ret.OdataBase = NewOdataBase(
		id,
		makeFullyQualifiedV1(rh, "$metadata#ComputerSystem.ComputerSystem"),
		"#ComputerSystem.v1_1_0.ComputerSystem",
		&odata,
		ret,
	)

    c.Members = append(c.Members, map[string]interface{}{"@odata.id": id})
}
