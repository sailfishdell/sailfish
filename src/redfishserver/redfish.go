package redfishserver

import (
	"encoding/json"
	"github.com/fatih/structs"

	"fmt"
)

var _ = fmt.Println

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
	OdataTree      *OdataTree
}

func rename(m map[string]interface{}, from string, to string) {
	m[to] = m[from]
	delete(m, from)
}

// Function to marshal the serviceroot properly
func (s *ServiceRoot) MarshalJSON() ([]byte, error) {
	m := structs.Map(s)
	delete(m, "Services")
	delete(m, "Collections")
	delete(m, "OdataTree")
	for k, v := range s.Services {
		m[k] = v
	}
	for k, v := range s.Collections {
		m[k] = v
	}
	rename(m, "OdataType", "@odata.type")
	rename(m, "OdataContext", "@odata.context")
	rename(m, "OdataID", "@odata.id")
	return json.Marshal(m)
}

func (s *ServiceRoot) AddCollection(name string, c *OdataCollection) {
	s.Collections[name] = c
	(*s.OdataTree)[c.OdataID] = c
}

type OdataCollection struct {
	OdataType    string `json:"@odata.type"`
	OdataContext string `json:"@odata.context"`
	OdataID      string `json:"@odata.id"`

	Name        string `json:"Name"`
	Description string `json:"Description"`
	MemberCount int    `json:"Members@odata.count"`
	Members     []interface{}
	Oem         map[string]interface{} `json:"Oem,omitempty"`
}

// Function to marshal the serviceroot properly
func (c *OdataCollection) MarshalJSON() ([]byte, error) {
	type Alias OdataCollection
	c.MemberCount = len(c.Members)
	return json.Marshal(&struct{ *Alias }{Alias: (*Alias)(c)})
}

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
