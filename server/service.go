package redfishserver

import (
	"context"
	"errors"
    "strings"
    eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"

    "github.com/superchalupa/go-rfs/domain"

    "fmt"
)

// Service is the business logic for a redfish server
type Service interface {
	RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (interface{}, error)
}

// ServiceMiddleware is a chainable behavior modifier for Service.
type ServiceMiddleware func(Service) Service

var (
	// ErrNotFound is returned when a request isnt present (404)
	ErrNotFound = errors.New("not found")
)

// Config is where we store the current service data
type config struct {
	baseURI   string
	verURI    string
	treeID    eh.UUID
	cmdbus    *commandbus.CommandBus
	odataRepo *repo.Repo
}

func (c *config) makeFullyQualifiedV1(path string) string {
	return c.baseURI + "/" + c.verURI + "/" + path
}

// NewService is how we initialize the business logic
func NewService(baseURI string, commandbus *commandbus.CommandBus, repo *repo.Repo, id eh.UUID) Service {
	cfg := config{baseURI: baseURI, verURI: "v1", cmdbus: commandbus, odataRepo: repo, treeID: id}
	go cfg.startup()
	return &cfg
}

func (rh *config) RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (output interface{}, err error) {
	noHashPath := strings.SplitN(url, "#", 2)[0]

    rawTree, err := rh.odataRepo.Find(ctx, rh.treeID)
    if err != nil {
        fmt.Printf("could not find tree: %s\n", err.Error())
		return nil, ErrNotFound
    }

    tree, ok := rawTree.(*domain.OdataTree)
    if !ok {
        fmt.Printf("somehow it wasnt a tree! %s\n", err.Error())
		return nil, ErrNotFound
    }

    requested, err := rh.odataRepo.Find(ctx, tree.Tree[noHashPath])
    if err != nil {
		return nil, ErrNotFound
    }
    item, ok := requested.(*domain.OdataItem)
    return item.Properties, nil
}

func (rh *config) startup() {
    ctx := context.Background()
    rh.createTreeLeaf(ctx, "/redfish/", map[string]interface{}{"v1": "/redfish/v1/"})
    rh.createTreeLeaf(ctx, "/redfish/v1/", map[string]interface{}{"Systems": "/redfish/v1/Systems"})
    rh.createTreeCollectionLeaf(ctx, "/redfish/v1/Systems", map[string]interface{}{"Systems": "/redfish/v1/Systems"})
}

func (rh *config) createTreeLeaf(ctx context.Context, uri string, Properties map[string]interface{} ){
	uuid := eh.NewUUID()
	rh.cmdbus.HandleCommand(ctx, &domain.CreateOdata{UUID: uuid, OdataURI: uri, Properties: Properties})
}

func (rh *config) createTreeCollectionLeaf(ctx context.Context, uri string, Properties map[string]interface{} ){
	uuid := eh.NewUUID()
	rh.cmdbus.HandleCommand(ctx, &domain.CreateOdataCollection{UUID: uuid, OdataURI: uri, Properties: Properties, Members: map[string]string{} })
}



