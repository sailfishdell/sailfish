package redfishserver

import (
	"context"
	"errors"
	eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"
	"strings"

	"github.com/superchalupa/go-rfs/domain"

	"fmt"
)

// Service is the business logic for a redfish server
type Service interface {
	GetOdataResource(ctx context.Context, headers map[string]string, url string, args map[string]string, privileges []string) (interface{}, error)
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

func (rh *config) GetOdataResource(ctx context.Context, headers map[string]string, url string, args map[string]string, privileges []string) (output interface{}, err error) {
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
	item, ok := requested.(*domain.OdataResource)
	return item.Properties, nil
}

func (rh *config) startup() {
	ctx := context.Background()

    // create version entry point. it's special in that it doesnt have @odata.* properties, so we'll remove them
    // after creating the object
	uuid := rh.createTreeLeaf(ctx, rh.baseURI+"/", "foo", "bar", map[string]interface{}{"v1": rh.makeFullyQualifiedV1("")})
	rh.cmdbus.HandleCommand(ctx, &domain.RemoveOdataResourceProperty{UUID: uuid, PropertyName: "@odata.context"})
	rh.cmdbus.HandleCommand(ctx, &domain.RemoveOdataResourceProperty{UUID: uuid, PropertyName: "@odata.id"})
	rh.cmdbus.HandleCommand(ctx, &domain.RemoveOdataResourceProperty{UUID: uuid, PropertyName: "@odata.type"})

	rh.createTreeLeaf(ctx, rh.makeFullyQualifiedV1(""),
		"#ServiceRoot.v1_0_2.ServiceRoot",
		rh.makeFullyQualifiedV1("$metadata#ServiceRoot"),
		map[string]interface{}{
			"Description":    "Root Service",
			"Id":             "RootService",
			"Name":           "Root Service",
			"RedfishVersion": "v1_0_2",
			"Systems":        map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("Systems")},
			"Chassis":        map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("Chassis")},
			"EventService":   map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("EventService")},
			"JsonSchemas":    map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("JSONSchemas")},
			"Managers":       map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("Managers")},
			"Registries":     map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("Registries")},
			"SessionService": map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("SessionService")},
			"Tasks":          map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("TaskService")},
			"UpdateService":  map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("UpdateService")},
			"Links": map[string]interface{}{
				"Sessions": map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("Sessions")},
			},
			"AccountService": map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("AccountService")},
		})
	rh.createTreeLeaf(ctx, rh.makeFullyQualifiedV1("EventService"),
		"#EventService.v1_0_2.EventService",
		rh.makeFullyQualifiedV1("$metadata#EventService"),
		map[string]interface{}{
			"EventTypesForSubscription@odata.count": 5,
			"Id":                           "EventService",
			"Name":                         "Event Service",
			"DeliveryRetryAttempts":        3,
			"DeliveryRetryIntervalSeconds": 30,
			"Description":                  "Event Service represents the properties for the service",
			"Subscriptions":                map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("EventService/Subscriptions")},
			"Status": map[string]interface{}{
				"Health":       "Ok",
				"HealthRollup": "Ok",
			},

			"Actions": map[string]interface{}{
				"#EventService.SubmitTestEvent": map[string]interface{}{
					"EventType@Redfish.AllowableValues": []string{
						"StatusChange",
						"ResourceUpdated",
						"ResourceAdded",
						"ResourceRemoved",
						"Alert",
					},
					"target": rh.makeFullyQualifiedV1("EventService/Actions/EventService.SubmitTestEvent"),
				},
			},

			"EventTypesForSubscription": []string{
				"StatusChange",
				"ResourceUpdated",
				"ResourceAdded",
				"ResourceRemoved",
				"Alert",
			},
		})
	rh.createTreeCollectionLeaf(ctx, rh.makeFullyQualifiedV1("Systems"), "#ComputerSystemCollection.ComputerSystemCollection", rh.makeFullyQualifiedV1("$metadata#Systems"), map[string]interface{}{}, []string{})
	rh.createTreeCollectionLeaf(ctx, rh.makeFullyQualifiedV1("Chassis"), "#ChassisCollection.ChassisCollection", rh.makeFullyQualifiedV1("$metadata#ChassisCollection"), map[string]interface{}{}, []string{})
	rh.createTreeCollectionLeaf(ctx, rh.makeFullyQualifiedV1("Managers"), "#ManagerCollection.ManagerCollection", rh.makeFullyQualifiedV1("$metadata#Managers"), map[string]interface{}{}, []string{})
	rh.createTreeCollectionLeaf(ctx, rh.makeFullyQualifiedV1("JSONSchemas"), "#JsonSchemaFileCollection.JsonSchemaFileCollection", rh.makeFullyQualifiedV1("$metadata#JsonSchemaFileCollection.JsonSchemaFileCollection"), map[string]interface{}{}, []string{})
	//rh.createTreeCollectionLeaf(ctx, rh.makeFullyQualifiedV1("Registries"), "unknown type", "unknown context", map[string]interface{}{}, []string{})
	rh.createTreeLeaf(ctx, rh.makeFullyQualifiedV1("AccountService"),
		"#AccountService.v1_0_2.AccountService",
		rh.makeFullyQualifiedV1("$metadata#AccountService"),
		map[string]interface{}{
			"@odata.type": "#AccountService.v1_0_2.AccountService",
			"Id":          "AccountService",
			"Name":        "Account Service",
			"Description": "Account Service",
			"Status": map[string]interface{}{
				"State":  "Enabled",
				"Health": "OK",
			},
			"ServiceEnabled":                  true,
			"AuthFailureLoggingThreshold":     3,
			"MinPasswordLength":               8,
			"AccountLockoutThreshold":         5,
			"AccountLockoutDuration":          30,
			"AccountLockoutCounterResetAfter": 30,
			"Accounts":                        map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("AccountService/Accounts")},
			"Roles":                           map[string]interface{}{"@odata.id": rh.makeFullyQualifiedV1("AccountService/Roles")},
			"@odata.context":                  rh.makeFullyQualifiedV1("$metadata#AccountService"),
			"@odata.id":                       rh.makeFullyQualifiedV1("AccountService"),
			"@Redfish.Copyright":              "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
		})

	rh.createTreeCollectionLeaf(ctx, rh.makeFullyQualifiedV1("AccountService/Accounts"),
		"#ManagerAccountCollection.ManagerAccountCollection",
		rh.makeFullyQualifiedV1("$metadata#ManagerAccountCollection.ManagerAccountCollection"),
		map[string]interface{}{
			"@odata.type":        "#ManagerAccountCollection.ManagerAccountCollection",
			"@odata.context":     rh.makeFullyQualifiedV1("$metadata#ManagerAccountCollection.ManagerAccountCollection"),
			"@odata.id":          rh.makeFullyQualifiedV1("AccountService/Accounts"),
			"@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
			"Name":               "Accounts Collection",
		},
		[]string{
			rh.makeFullyQualifiedV1("AccountService/Accounts/1"),
			rh.makeFullyQualifiedV1("AccountService/Accounts/2"),
			rh.makeFullyQualifiedV1("AccountService/Accounts/3"),
			rh.makeFullyQualifiedV1("AccountService/Accounts/4"),
		})
	rh.createTreeLeaf(ctx, rh.makeFullyQualifiedV1("AccountService/Accounts/1"),
		"#ManagerAccount.v1_0_2.ManagerAccount",
		rh.makeFullyQualifiedV1("$metadata#ManagerAccount.ManagerAccount"),
		map[string]interface{}{
			"@odata.type":        "#ManagerAccount.v1_0_2.ManagerAccount",
			"@odata.context":     rh.makeFullyQualifiedV1("$metadata#ManagerAccount.ManagerAccount"),
			"@odata.id":          rh.makeFullyQualifiedV1("AccountService/Accounts/1"),
			"@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
			"Id":                 "1",
			"Name":               "User Account",
			"Description":        "User Account",
			"Enabled":            true,
			"Password":           nil,
			"UserName":           "Administrator",
			"RoleId":             "Admin",
			"Locked":             false,
			"Links": map[string]interface{}{
				"Role": map[string]interface{}{
					"@odata.id": rh.makeFullyQualifiedV1("AccountService/Roles/Admin"),
				},
			},
		})
	rh.createTreeCollectionLeaf(ctx, rh.makeFullyQualifiedV1("AccountService/Roles"),
		"#RoleCollection.RoleCollection",
		rh.makeFullyQualifiedV1("$metadata#Role.Role"),
		map[string]interface{}{
			"@odata.type":        "#RoleCollection.RoleCollection",
			"Name":               "Roles Collection",
			"@odata.context":     rh.makeFullyQualifiedV1("$metadata#Role.Role"),
			"@odata.id":          rh.makeFullyQualifiedV1("AccountService/Roles"),
			"@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
		},
		[]string{
			rh.makeFullyQualifiedV1("AccountService/Roles/ReadOnlyUser"),
			rh.makeFullyQualifiedV1("AccountService/Roles/Operator"),
			rh.makeFullyQualifiedV1("AccountService/Roles/Admin"),
		})
	rh.createTreeLeaf(ctx, rh.makeFullyQualifiedV1("AccountService/Roles/Admin"),
		"#Role.v1_0_2.Role",
		rh.makeFullyQualifiedV1("$metadata#Role.Role"),
		map[string]interface{}{
			"@odata.context":     rh.makeFullyQualifiedV1("$metadata#Role.Role"),
			"@odata.id":          rh.makeFullyQualifiedV1("AccountService/Roles/Admin"),
			"@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
			"@odata.type":        "#Role.v1_0_2.Role",
			"Id":                 "Admin",
			"Name":               "User Role",
			"Description":        "Admin User Role",
			"IsPredefined":       true,
			"AssignedPrivileges": []string{
				"Login",
				"ConfigureManager",
				"ConfigureUsers",
				"ConfigureSelf",
				"ConfigureComponents",
			},
			"OEMPrivileges": []string{
				"OemClearLog",
				"OemPowerControl",
			},
		})
}

func (rh *config) createTreeLeaf(ctx context.Context, uri string, otype string, octx string, Properties map[string]interface{}) (uuid eh.UUID) {
	uuid = eh.NewUUID()
	fmt.Printf("Creating URI %s\n", uri)
	rh.cmdbus.HandleCommand(ctx, &domain.CreateOdataResource{UUID: uuid, ResourceURI: uri, Properties: Properties, Type: otype, Context: octx})
    return
}

func (rh *config) createTreeCollectionLeaf(ctx context.Context, uri string, otype string, octx string, Properties map[string]interface{}, Members []string) {
	uuid := eh.NewUUID()
	rh.cmdbus.HandleCommand(ctx, &domain.CreateOdataResourceCollection{UUID: uuid, ResourceURI: uri, Properties: Properties, Members: Members, Type: otype, Context: octx})
}
