package redfishserver

import (
	"context"
	"encoding/json"
	"fmt"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/domain"
	"io"
	"os"
	"path"
	"strings"
)

type Ingester interface {
	FromOdataID(odataid string) (io.ReadCloser, error)
}

//****************************************************************************
// File Ingester
//****************************************************************************

type spmfIngester struct {
	basePath string
}

func NewSPMFIngester(basePath string) spmfIngester {
	return spmfIngester{basePath: basePath}
}

func (i spmfIngester) FromOdataID(odataid string) (f io.ReadCloser, err error) {
	id := strings.SplitN(odataid, "#", 2)[0]
	id = strings.Replace(id, "/redfish/v1/", "", -1)
	p := path.Join(i.basePath, id+"/index.json")
	fmt.Printf("OPENING: =>%s<=\n", p)
	f, err = os.Open(p)
	return
}

// End SPMF Ingester
//****************************************************************************

func Ingest(i Ingester, d domain.DDDFunctions, odataid string) error {
	fmt.Printf("Ingesting @odata.id = %s\n", odataid)

	// get the odata tree
	ctx := context.Background()
	tree, err := domain.GetTree(ctx, d.GetReadRepo(), d.GetTreeID())

	idExists := func(id string) bool {
		if tree == nil {
			return false
		}
		_, ok := tree.Tree[id]
		return ok
	}

	var properties map[string]interface{}
	f, err := i.FromOdataID(odataid)
	if err != nil {
		fmt.Printf("Error FromOdataID: %s\n", err.Error())
		return err
	}
	err = json.NewDecoder(f).Decode(&properties)
	if err != nil {
		fmt.Printf("Error from json decode: %s\n", err.Error())
		return err
	}

	err = nil
	if idExists(odataid) {
		err = updateRedfishResource(ctx, d, tree.Tree[odataid], properties)
	} else {
		err = createTreeLeaf(ctx, d, odataid, properties)
	}
	if err != nil {
		return err
	}

	subids := getNestedOdataIds(properties, true)
	for _, id := range subids {
		// prevent loops, only import if not already imported
		if !idExists(id) {
			err := Ingest(i, d, id)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func createTreeLeaf(ctx context.Context, d domain.DDDFunctions, uri string, Properties map[string]interface{}) error {
	uuid := eh.NewUUID()
	fmt.Printf("Creating URI %s at %s\n", uri, uuid)
	c := &domain.CreateRedfishResource{
		RedfishResourceAggregateBaseCommand: domain.RedfishResourceAggregateBaseCommand{UUID: uuid},
		ResourceURI:                         uri,
		Properties:                          Properties,
		Type:                                Properties["@odata.type"].(string),
		Context:                             Properties["@odata.context"].(string),
	}
	err := d.GetCommandBus().HandleCommand(ctx, c)
	if err != nil {
		return err
	}
	return nil
}

func updateRedfishResource(ctx context.Context, d domain.DDDFunctions, uuid eh.UUID, Properties map[string]interface{}) error {
	fmt.Printf("updateRedfishResource: %s\n", Properties)
	c := &domain.UpdateRedfishResourceProperties{RedfishResourceAggregateBaseCommand: domain.RedfishResourceAggregateBaseCommand{UUID: uuid}, Properties: Properties}
	err := d.GetCommandBus().HandleCommand(ctx, c)
	if err != nil {
		return err
	}
	return nil
}

func getNestedOdataIds(inputJSON interface{}, allowonce bool) (output []string) {
	var nestedIds []string

	// range over input, copying to output
	switch nested := inputJSON.(type) {
	case map[string]interface{}:
		if _, ok := nested["@odata.id"]; ok && (!allowonce) {
			uri, ok := nested["@odata.id"].(string)
			if ok {
				uri = strings.SplitN(uri, "#", 2)[0]
				nestedIds = append(nestedIds, uri)
			}
		}

		for _, v := range nested {
			nestedIds = append(nestedIds, getNestedOdataIds(v, false)...)
		}

	case []interface{}:
		for _, mem := range nested {
			nestedIds = append(nestedIds, getNestedOdataIds(mem, false)...)
		}
	}
	return nestedIds
}
