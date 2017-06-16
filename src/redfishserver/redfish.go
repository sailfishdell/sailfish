package redfishserver

import ()

/*
type OdataCollection struct {
	Name        string `json:"Name"`
	Description string `json:"Description"`
	MemberCount string `json:"Members@odata.count"`
	Members     []interface{}
	Oem         map[string]interface{}
    OdataBase
}
*/

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

/*
{
  "@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
  "@odata.context": "/redfish/v1/$metadata#ServiceRoot",
  "@odata.id": "/redfish/v1/",
  "@odata.type": "#ServiceRoot.v1_0_2.ServiceRoot",
  "RedfishVersion": "1.0.2",
  "Name": "Root Service",
  "UUID": "92384634-2938-2342-8820-489239905423",
  "Id": "RootService",

  "AccountService": {
    "@odata.id": "/redfish/v1/AccountService"
  },
  "Chassis": {
    "@odata.id": "/redfish/v1/Chassis"
  },
  "EventService": {
    "@odata.id": "/redfish/v1/EventService"
  },
  "Links": {
    "Sessions": {
      "@odata.id": "/redfish/v1/SessionService/Sessions"
    }
  },
  "Managers": {
    "@odata.id": "/redfish/v1/Managers"
  },
  "Oem": {},
  "SessionService": {
    "@odata.id": "/redfish/v1/SessionService"
  },
  "Systems": {
    "@odata.id": "/redfish/v1/Systems"
  },
  "Tasks": {
    "@odata.id": "/redfish/v1/TaskService"
  },
  "madness": {
    "msg": "LETS GO CRAZY 2596996162 TIMES"
  }
}

*/
