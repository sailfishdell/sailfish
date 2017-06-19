package redfishserver

import (
	"context"
	"encoding/json"
	"github.com/fatih/structs"

	"fmt"
	"reflect"
)

var _ = fmt.Println
var _ = json.Marshal

/*
   tree node
       serialize(query/filter/select)
       Add node pointer
       delete node pointer
*/

type OdataTree map[string]interface{}

type OdataTreeInt interface {
	Serialize(context.Context) (map[string]interface{}, error)
}

type OdataBase struct {
	OdataType    string `json:"@odata.type"`
	OdataContext string `json:"@odata.context"`
	OdataID      string `json:"@odata.id"`
	Nodes        map[string]OdataBase
	OdataTree    *OdataTree
	wrapper      interface{}
}

func (o *OdataBase) Serialize(ctx context.Context) (map[string]interface{}, error) {
	fmt.Println("DEBUG: ", reflect.TypeOf(o.wrapper))
	m := structs.Map(o.wrapper)

	rename := func(base, from, to string) {
		m[to] = m[base].(map[string]interface{})[from]
		delete(m[base].(map[string]interface{}), from)
	}

	rename("OdataBase", "OdataType", "@odata.type")
	rename("OdataBase", "OdataContext", "@odata.context")
	rename("OdataBase", "OdataID", "@odata.id")
	delete(m, "OdataBase")

	//	// collapse Collections to just the link to the collection
	//	for k, v := range m.Nodes {
	//		if id, ok := v.(*Nodes); ok {
	//			m[k] = map[string]interface{}{"@odata.id": id.OdataID}
	//		}
	//	}

	return m, nil
}

type ServiceRoot struct {
	RedfishVersion string
	Id             string
	Name           string
	Description    string
	*OdataBase
}
