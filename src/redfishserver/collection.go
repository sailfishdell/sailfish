package redfishserver

import ()

type OdataCollection struct {
	Type        string `json:"@odata.type"`
	Context     string `json:"@odata.context"`
	ID          string `json:"@odata.id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	MemberCount string `json:"Members@odata.count"`
	Members     []interface{}
	Oem         map[string]interface{}
}


/*
func (OdataCollection) MarshalJSON() ([]byte, error) {
	outstr := fmt.Sprintf(`{"msg": "LETS GO CRAZY %d TIMES"}`, rand.Uint32())
	buffer := bytes.NewBufferString(outstr)
	return buffer.Bytes(), nil
}
*/


/*
{
    "@odata.type": "#SerialInterfaceCollection.SerialInterfaceCollection",
    "Name": "Serial Interface Collection",
    "Description": "Collection of Serial Interfaces for this System",
    "Members@odata.count": 1,
    "Members": [
        {
            "@odata.id": "/redfish/v1/Managers/BMC/SerialInterfaces/TTY0"
        }
    ],
    "Oem": {},
    "@odata.context": "/redfish/v1/$metadata#Managers/BMC/SerialInterfaces/$entity",
    "@odata.id": "/redfish/v1/Managers/BMC/SerialInterfaces",
    "@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright."
}
*/
