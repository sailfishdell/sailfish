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
