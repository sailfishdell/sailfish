package redfishserver

import (
	"context"
	"errors"
	eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"
	"github.com/superchalupa/eventhorizon/utils"
	"net/http"
	"strings"
	"time"

	"github.com/superchalupa/go-rfs/domain"

	"fmt"
)

type Response struct {
	// status code is for external users
	StatusCode int
	Headers    map[string]string
	Output     interface{}
}

// Service is the business logic for a redfish server
type Service interface {
	GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error)
	RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error)
	MakeFullyQualifiedV1(string) string
	GetBaseURI() string
	GetTreeID() eh.UUID
	GetCommandBus() *commandbus.CommandBus
	GetEventHandler() eh.EventHandler
	GetRepo() *repo.Repo
	GetEventWaiter() *utils.EventWaiter
	FindUser(ctx context.Context, user string) (account *domain.RedfishResource, err error)
	GetPrivileges(ctx context.Context, account *domain.RedfishResource) (privileges []string)
}

// ServiceMiddleware is a chainable behavior modifier for Service.
type ServiceMiddleware func(Service) Service

var (
	// ErrNotFound is returned when a request isnt present (404)
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("Unauthorized") // 401... missing or bad authentication
	ErrForbidden    = errors.New("Forbidden")    // should be 403 (you are authenticated, but dont have permissions to this object)
)

// ServiceConfig is where we store the current service data
type ServiceConfig struct {
	baseURI      string
	verURI       string
	treeID       eh.UUID
	cmdbus       *commandbus.CommandBus
	eventHandler eh.EventHandler
	redfishRepo  *repo.Repo
	waiter       *utils.EventWaiter
	httpsagas    *domain.HTTPSagaList
}

func (c *ServiceConfig) MakeFullyQualifiedV1(path string) string {
	return c.baseURI + "/" + c.verURI + "/" + path
}

func (c *ServiceConfig) GetBaseURI() string {
	return c.baseURI
}

func (c *ServiceConfig) GetTreeID() eh.UUID {
	return c.treeID
}

func (c *ServiceConfig) GetCommandBus() *commandbus.CommandBus {
	return c.cmdbus
}

func (c *ServiceConfig) GetEventHandler() eh.EventHandler {
	return c.eventHandler
}

func (c *ServiceConfig) GetRepo() *repo.Repo {
	return c.redfishRepo
}

func (c *ServiceConfig) GetEventWaiter() *utils.EventWaiter {
	return c.waiter
}

func (s *ServiceConfig) FindUser(ctx context.Context, user string) (account *domain.RedfishResource, err error) {
	// start looking up user in auth service
	tree, err := domain.GetTree(ctx, s.GetRepo(), s.GetTreeID())
	if err != nil {
		return nil, errors.New("Malformed tree")
	}

	// get the root service reference
	rootService, err := tree.GetRedfishResourceFromTree(ctx, s.GetRepo(), s.MakeFullyQualifiedV1(""))
	if err != nil {
		return nil, errors.New("Malformed tree root resource")
	}

	// Pull up the Accounts Collection
	accounts, err := tree.WalkRedfishResourceTree(ctx, s.GetRepo(), rootService, "AccountService", "@odata.id", "Accounts", "@odata.id")
	if err != nil {
		return nil, errors.New("Malformed Account Service")
	}

	// Walk through all of the "Members" of the collection, which are links to individual accounts
	members, ok := accounts.Properties["Members"]
	if !ok {
		return nil, errors.New("Malformed Account Collection")
	}

	// avoid panics by separating out type assertion
	memberList, ok := members.([]map[string]interface{})
	if !ok {
		return nil, errors.New("Malformed Account Collection")
	}

	for _, m := range memberList {
		a, _ := tree.GetRedfishResourceFromTree(ctx, s.GetRepo(), m["@odata.id"].(string))
		if a == nil {
			continue
		}
		if a.Properties == nil {
			continue
		}
		memberUser, ok := a.Properties["UserName"]
		if !ok {
			continue
		}
		if memberUser != user {
			continue
		}
		return a, nil
	}
	return nil, errors.New("User not found")
}

func (s *ServiceConfig) GetPrivileges(ctx context.Context, account *domain.RedfishResource) (privileges []string) {
	// start looking up user in auth service
	tree, err := domain.GetTree(ctx, s.GetRepo(), s.GetTreeID())
	if err != nil {
		return
	}

	role, _ := tree.WalkRedfishResourceTree(ctx, s.GetRepo(), account, "Links", "Role", "@odata.id")
	privs, ok := role.Properties["AssignedPrivileges"]
	if !ok {
		return
	}

	for _, p := range privs.([]string) {
		// If the user has "ConfigureSelf", then append the special privilege that lets them configure their specific attributes
		if p == "ConfigureSelf" {
			// Add ConfigureSelf_%{USERNAME} property
			privileges = append(privileges, "ConfigureSelf_"+account.Properties["UserName"].(string))
		} else {
			// otherwise just pass through the actual priv
			privileges = append(privileges, p)
		}
	}

	var _ = fmt.Printf
	//fmt.Printf("Assigned the following Privileges: %s\n", privileges)
	return
}

// NewService is how we initialize the business logic
func NewService(baseURI string, commandbus *commandbus.CommandBus, eventHandler eh.EventHandler, repo *repo.Repo, id eh.UUID, w *utils.EventWaiter) Service {
	cfg := ServiceConfig{
		baseURI:      baseURI,
		verURI:       "v1",
		cmdbus:       commandbus,
		redfishRepo:  repo,
		treeID:       id,
		waiter:       w,
		httpsagas:    domain.NewHTTPSagaList(commandbus, repo, eventHandler, baseURI),
		eventHandler: eventHandler,
	}
	go cfg.startup()
	return &cfg
}

func (rh *ServiceConfig) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	noHashPath := strings.SplitN(r.URL.Path, "#", 2)[0]

	// we have the tree ID, fetch an updated copy of the actual tree
	// TODO: Locking? Should repo give us a copy? Need to test this.
	tree, err := domain.GetTree(ctx, rh.redfishRepo, rh.treeID)
	if err != nil {
		return &Response{StatusCode: http.StatusInternalServerError}, err
	}

	// now that we have the tree, look up the actual URI in that tree to find
	// the object UUID, then pull that from the repo
	requested, err := rh.redfishRepo.Find(ctx, tree.Tree[noHashPath])
	if err != nil {
		return &Response{StatusCode: http.StatusNotFound}, nil
	}
	item, ok := requested.(*domain.RedfishResource)
	if !ok {
		return &Response{StatusCode: http.StatusInternalServerError}, errors.New("Expected a RedfishResource, but got something strange.")
	}

	return &Response{StatusCode: http.StatusOK, Output: item.Properties, Headers: item.Headers}, nil
}

func (rh *ServiceConfig) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	// we shouldn't actually ever get a path with a hash, I don't think.
	noHashPath := strings.SplitN(r.URL.Path, "#", 2)[0]

	// we have the tree ID, fetch an updated copy of the actual tree
	// TODO: Locking? Should repo give us a copy? Need to test this.
	tree, err := domain.GetTree(ctx, rh.redfishRepo, rh.treeID)
	if err != nil {
		return &Response{StatusCode: http.StatusInternalServerError}, err
	}

	// now that we have the tree, look up the actual URI in that tree to find
	// the object UUID, then pull that from the repo
	requested, err := rh.redfishRepo.Find(ctx, tree.Tree[noHashPath])
	if err != nil {
		// it's ok if obj not found
		return &Response{StatusCode: http.StatusNotFound}, nil
	}
	item, ok := requested.(*domain.RedfishResource)
	if !ok {
		return &Response{StatusCode: http.StatusInternalServerError}, errors.New("Expected a RedfishResource, but got something strange.")
	}

	cmdUUID := eh.NewUUID()

	// we send a command and then wait for a completion event. Set up the wait here.
	waitID, resultChan := rh.waiter.SetupWait(func(event eh.Event) bool {
		if event.EventType() != domain.HTTPCmdProcessedEvent {
			return false
		}
		if data, ok := event.Data().(*domain.HTTPCmdProcessedData); ok {
			if data.CommandID == cmdUUID {
				return true
			}
		}
		return false
	})

	defer rh.waiter.CancelWait(waitID)

	// Check for single COMMAND that can be run
	// look up to see if there is a specific command
	err = rh.httpsagas.RunHTTPOperation(ctx, rh.treeID, cmdUUID, item, r)
	if err != nil {
		return &Response{StatusCode: http.StatusMethodNotAllowed}, nil
	}

	select {
	case event := <-resultChan:
		d := event.Data().(*domain.HTTPCmdProcessedData)
		return &Response{Output: d.Results, StatusCode: d.StatusCode, Headers: d.Headers}, nil
	case <-time.After(1 * time.Second):
		// TODO: Here we could easily automatically create a JOB and return that.
		return &Response{StatusCode: http.StatusOK, Output: "JOB"}, nil
	case <-ctx.Done():
		// the requestor cancelled the http request to us. We can abandon
		// returning results, but command will still be processed
		return &Response{StatusCode: http.StatusBadRequest}, nil
	}
}

func (rh *ServiceConfig) startup() {
	ctx := context.Background()

	// create version entry point. it's special in that it doesnt have @odata.* properties, so we'll remove them
	// after creating the object
	uuid := rh.createTreeLeaf(ctx, rh.baseURI+"/", "foo", "bar", map[string]interface{}{"v1": rh.MakeFullyQualifiedV1("")})
	rh.cmdbus.HandleCommand(ctx, &domain.RemoveRedfishResourceProperty{UUID: uuid, PropertyName: "@odata.context"})
	rh.cmdbus.HandleCommand(ctx, &domain.RemoveRedfishResourceProperty{UUID: uuid, PropertyName: "@odata.id"})
	rh.cmdbus.HandleCommand(ctx, &domain.RemoveRedfishResourceProperty{UUID: uuid, PropertyName: "@odata.type"})

	rh.createTreeLeaf(ctx, rh.MakeFullyQualifiedV1(""),
		"#ServiceRoot.v1_0_2.ServiceRoot",
		rh.MakeFullyQualifiedV1("$metadata#ServiceRoot"),
		map[string]interface{}{
			"Description":    "Root Service",
			"Id":             "RootService",
			"Name":           "Root Service",
			"RedfishVersion": "v1_0_2",
			"Systems":        map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("Systems")},
			"Chassis":        map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("Chassis")},
			"EventService":   map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("EventService")},
			"JsonSchemas":    map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("JSONSchemas")},
			"Managers":       map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("Managers")},
			"Registries":     map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("Registries")},
			"SessionService": map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("SessionService")},
			"Tasks":          map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("TaskService")},
			"UpdateService":  map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("UpdateService")},
			"Links": map[string]interface{}{
				"Sessions": map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("SessionService/Sessions")},
			},
			"AccountService": map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("AccountService")},
		})
	rh.createTreeLeaf(ctx, rh.MakeFullyQualifiedV1("SessionService"),
		"#SessionService.v1_0_2.SessionService",
		rh.MakeFullyQualifiedV1("$metadata#SessionService"),
		map[string]interface{}{
			"Id":          "SessionService",
			"Name":        "Session Service",
			"Description": "Session Service",
			"Status": map[string]interface{}{
				"State":  "Enabled",
				"Health": "OK",
			},
			"ServiceEnabled": true,
			"SessionTimeout": 30,
			"Sessions": map[string]interface{}{
				"@odata.id": rh.MakeFullyQualifiedV1("SessionService/Sessions"),
			},
		})
	rh.createTreeCollectionLeaf(ctx, rh.MakeFullyQualifiedV1("SessionService/Sessions"),
		"#SessionCollection.SessionCollection",
		rh.MakeFullyQualifiedV1("$metadata#SessionService/Sessions/$entity"),
		map[string]interface{}{},
		[]string{},
	)

	rh.createTreeLeaf(ctx, rh.MakeFullyQualifiedV1("EventService"),
		"#EventService.v1_0_2.EventService",
		rh.MakeFullyQualifiedV1("$metadata#EventService"),
		map[string]interface{}{
			"EventTypesForSubscription@odata.count": 5,
			"Id":                           "EventService",
			"Name":                         "Event Service",
			"DeliveryRetryAttempts":        3,
			"DeliveryRetryIntervalSeconds": 30,
			"Description":                  "Event Service represents the properties for the service",
			"Subscriptions":                map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("EventService/Subscriptions")},
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
					"target": rh.MakeFullyQualifiedV1("EventService/Actions/EventService.SubmitTestEvent"),
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
	rh.createTreeCollectionLeaf(ctx, rh.MakeFullyQualifiedV1("Systems"), "#ComputerSystemCollection.ComputerSystemCollection", rh.MakeFullyQualifiedV1("$metadata#Systems"), map[string]interface{}{}, []string{})
	rh.createTreeCollectionLeaf(ctx, rh.MakeFullyQualifiedV1("Chassis"), "#ChassisCollection.ChassisCollection", rh.MakeFullyQualifiedV1("$metadata#ChassisCollection"), map[string]interface{}{}, []string{})
	rh.createTreeCollectionLeaf(ctx, rh.MakeFullyQualifiedV1("Managers"), "#ManagerCollection.ManagerCollection", rh.MakeFullyQualifiedV1("$metadata#Managers"), map[string]interface{}{}, []string{})
	rh.createTreeCollectionLeaf(ctx, rh.MakeFullyQualifiedV1("JSONSchemas"), "#JsonSchemaFileCollection.JsonSchemaFileCollection", rh.MakeFullyQualifiedV1("$metadata#JsonSchemaFileCollection.JsonSchemaFileCollection"), map[string]interface{}{}, []string{})
	//rh.createTreeCollectionLeaf(ctx, rh.MakeFullyQualifiedV1("Registries"), "unknown type", "unknown context", map[string]interface{}{}, []string{})
	rh.createTreeLeaf(ctx, rh.MakeFullyQualifiedV1("AccountService"),
		"#AccountService.v1_0_2.AccountService",
		rh.MakeFullyQualifiedV1("$metadata#AccountService"),
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
			"Accounts":                        map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("AccountService/Accounts")},
			"Roles":                           map[string]interface{}{"@odata.id": rh.MakeFullyQualifiedV1("AccountService/Roles")},
			"@odata.context":                  rh.MakeFullyQualifiedV1("$metadata#AccountService"),
			"@odata.id":                       rh.MakeFullyQualifiedV1("AccountService"),
			"@Redfish.Copyright":              "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
		})

	rh.createTreeCollectionLeaf(ctx, rh.MakeFullyQualifiedV1("AccountService/Accounts"),
		"#ManagerAccountCollection.ManagerAccountCollection",
		rh.MakeFullyQualifiedV1("$metadata#ManagerAccountCollection.ManagerAccountCollection"),
		map[string]interface{}{
			"@odata.type":        "#ManagerAccountCollection.ManagerAccountCollection",
			"@odata.context":     rh.MakeFullyQualifiedV1("$metadata#ManagerAccountCollection.ManagerAccountCollection"),
			"@odata.id":          rh.MakeFullyQualifiedV1("AccountService/Accounts"),
			"@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
			"Name":               "Accounts Collection",
		},
		[]string{
			rh.MakeFullyQualifiedV1("AccountService/Accounts/1"),
			rh.MakeFullyQualifiedV1("AccountService/Accounts/2"),
			rh.MakeFullyQualifiedV1("AccountService/Accounts/3"),
			rh.MakeFullyQualifiedV1("AccountService/Accounts/4"),
		})
	rh.createTreeLeaf(ctx, rh.MakeFullyQualifiedV1("AccountService/Accounts/1"),
		"#ManagerAccount.v1_0_2.ManagerAccount",
		rh.MakeFullyQualifiedV1("$metadata#ManagerAccount.ManagerAccount"),
		map[string]interface{}{
			"@odata.type":        "#ManagerAccount.v1_0_2.ManagerAccount",
			"@odata.context":     rh.MakeFullyQualifiedV1("$metadata#ManagerAccount.ManagerAccount"),
			"@odata.id":          rh.MakeFullyQualifiedV1("AccountService/Accounts/1"),
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
					"@odata.id": rh.MakeFullyQualifiedV1("AccountService/Roles/Admin"),
				},
			},
		})
	rh.createTreeCollectionLeaf(ctx, rh.MakeFullyQualifiedV1("AccountService/Roles"),
		"#RoleCollection.RoleCollection",
		rh.MakeFullyQualifiedV1("$metadata#Role.Role"),
		map[string]interface{}{
			"@odata.type":        "#RoleCollection.RoleCollection",
			"Name":               "Roles Collection",
			"@odata.context":     rh.MakeFullyQualifiedV1("$metadata#Role.Role"),
			"@odata.id":          rh.MakeFullyQualifiedV1("AccountService/Roles"),
			"@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
		},
		[]string{
			rh.MakeFullyQualifiedV1("AccountService/Roles/ReadOnlyUser"),
			rh.MakeFullyQualifiedV1("AccountService/Roles/Operator"),
			rh.MakeFullyQualifiedV1("AccountService/Roles/Admin"),
		})
	rh.createTreeLeaf(ctx, rh.MakeFullyQualifiedV1("AccountService/Roles/Admin"),
		"#Role.v1_0_2.Role",
		rh.MakeFullyQualifiedV1("$metadata#Role.Role"),
		map[string]interface{}{
			"@odata.context":     rh.MakeFullyQualifiedV1("$metadata#Role.Role"),
			"@odata.id":          rh.MakeFullyQualifiedV1("AccountService/Roles/Admin"),
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

func (rh *ServiceConfig) createTreeLeaf(ctx context.Context, uri string, otype string, octx string, Properties map[string]interface{}) (uuid eh.UUID) {
	uuid = eh.NewUUID()
	fmt.Printf("Creating URI %s\n", uri)
	rh.cmdbus.HandleCommand(ctx, &domain.CreateRedfishResource{UUID: uuid, ResourceURI: uri, Properties: Properties, Type: otype, Context: octx})
	return
}

func (rh *ServiceConfig) createTreeCollectionLeaf(ctx context.Context, uri string, otype string, octx string, Properties map[string]interface{}, Members []string) {
	uuid := eh.NewUUID()
	rh.cmdbus.HandleCommand(ctx, &domain.CreateRedfishResourceCollection{UUID: uuid, ResourceURI: uri, Properties: Properties, Members: Members, Type: otype, Context: octx})
}
