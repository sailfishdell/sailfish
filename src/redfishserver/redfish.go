package redfishserver

import (
	"context"
	"github.com/fatih/structs"
)

type Wrapable interface {
	GetWrapper() interface{}
}

type OdataTreeInt interface {
	AddNode(string, OdataTreeInt)
	GetBase() OdataTreeInt
	Wrapable
	OdataSerializable
}

type OdataBase struct {
	OdataType    string `json:"@odata.type"`
	OdataContext string `json:"@odata.context"`
	OdataID      string `json:"@odata.id"`
	Nodes        map[string]OdataTreeInt
	OdataTree    *OdataTree
	wrapper      interface{}
}

func NewOdataBase(id, context, otype string, t *OdataTree, w OdataSerializable) *OdataBase {
	t.SetBody(id, w)
	return &OdataBase{
		OdataType:    otype,
		OdataContext: context,
		OdataID:      id,
		Nodes:        map[string]OdataTreeInt{},
		OdataTree:    t,
		wrapper:      w,
	}
}

func (o *OdataBase) AddNode(s string, p OdataTreeInt) {
	o.Nodes[s] = p
}

func (o *OdataBase) GetBase() OdataTreeInt {
	return o
}

func (o *OdataBase) GetWrapper() interface{} {
	return o.wrapper
}

func (o *OdataBase) OdataSerialize(ctx context.Context) (map[string]interface{}, error) {
	m := structs.Map(o.wrapper)

	rename := func(base, from, to string) {
		m[to] = m[base].(map[string]interface{})[from]
		delete(m[base].(map[string]interface{}), from)
	}

	rename("OdataBase", "OdataType", "@odata.type")
	rename("OdataBase", "OdataContext", "@odata.context")
	rename("OdataBase", "OdataID", "@odata.id")
	delete(m, "OdataBase")

	// collapse Collections to just the link to the collection
	for k, v := range o.Nodes {
		m[k] = map[string]interface{}{"@odata.id": v.GetBase().(*OdataBase).OdataID}
	}

	return m, nil
}

type ServiceRoot struct {
	RedfishVersion string
	Id             string
	Name           string
	Description    string
	*OdataBase
}

type Collection struct {
	Name    string
	Members []map[string]interface{}
	*OdataBase
}

func (c *Collection) OdataSerialize(ctx context.Context) (map[string]interface{}, error) {
	m, err := c.OdataBase.OdataSerialize(ctx)
	m["Members@odata.count"] = len(c.Members)
	return m, err
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
