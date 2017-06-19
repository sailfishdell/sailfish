package redfishserver

import (
	"encoding/json"
	"github.com/fatih/structs"
    "context"

	"fmt"
    "reflect"
)

var _ = fmt.Println


/*
    tree node
        serialize(query/filter/select)
        Add node pointer
        delete node pointer
*/



type OdataTreeInt interface{
    Serialize(context.Context) (map[string]interface{}, error)
}

type OdataBase struct {
	OdataType      string `json:"@odata.type"`
	OdataContext   string `json:"@odata.context"`
	OdataID        string `json:"@odata.id"`
    wrapper interface{}
}

func (o *OdataBase) Serialize(ctx context.Context, ) (map[string]interface{}, error) {
    fmt.Println("DEBUG: ", reflect.TypeOf(o.wrapper))
	m := structs.Map(o.wrapper)
	//rename(m, "OdataType", "@odata.type")
	//rename(m, "OdataContext", "@odata.context")
	//rename(m, "OdataID", "@odata.id")
    return m, nil
}


type SR2 struct {
	RedfishVersion string
    *OdataBase
}




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
	// collapse Collections to just the link to the collection
	for k, v := range s.Collections {
		if id, ok := v.(*OdataCollection); ok {
			m[k] = map[string]interface{}{"@odata.id": id.OdataID}
		}
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

type BaseOdataID struct {
	OdataID string `json:"@odata.id"`
}

type OdataCollection struct {
	OdataType    string `json:"@odata.type"`
	OdataContext string `json:"@odata.context"`

	Name        string `json:"Name"`
	Description string `json:"Description"`
	MemberCount int    `json:"Members@odata.count"`
	Members     []interface{}
	Oem         map[string]interface{} `json:"Oem,omitempty"`
	BaseOdataID
}

// Function to marshal the serviceroot properly
func (c *OdataCollection) MarshalJSON() ([]byte, error) {
	type Alias OdataCollection
	c.MemberCount = len(c.Members)
	return json.Marshal(&struct{ *Alias }{Alias: (*Alias)(c)})
}

type OdataTree map[string]interface{}
