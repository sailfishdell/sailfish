package domain

import (
	"context"
	"errors"
	"fmt"
	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/utils"
)

var _ = fmt.Println

type DDDFunctions interface {
	MakeFullyQualifiedV1(string) string
	GetBaseURI() string
	GetTreeID() eh.UUID
	GetCommandBus() eh.CommandBus
	GetEventHandler() eh.EventHandler
	GetRepo() eh.ReadRepo
	GetEventWaiter() *utils.EventWaiter
}

type baseDDD struct {
	baseURI      string
	verURI       string
	treeID       eh.UUID
	cmdbus       eh.CommandBus
	eventHandler eh.EventHandler
	redfishRepo  eh.ReadRepo
	waiter       *utils.EventWaiter
}

// NewService is how we initialize the business logic
func NewBaseDDD(baseURI string, commandbus eh.CommandBus, eventHandler eh.EventHandler, repo eh.ReadRepo, id eh.UUID, w *utils.EventWaiter) DDDFunctions {
	return &baseDDD{
		baseURI:      baseURI,
		verURI:       "v1",
		cmdbus:       commandbus,
		redfishRepo:  repo,
		treeID:       id,
		waiter:       w,
		eventHandler: eventHandler,
	}
}

func (c *baseDDD) MakeFullyQualifiedV1(path string) string {
	return c.baseURI + "/" + c.verURI + "/" + path
}

func (c *baseDDD) GetBaseURI() string {
	return c.baseURI
}

func (c *baseDDD) GetTreeID() eh.UUID {
	return c.treeID
}

func (c *baseDDD) GetCommandBus() eh.CommandBus {
	return c.cmdbus
}

func (c *baseDDD) GetEventHandler() eh.EventHandler {
	return c.eventHandler
}

func (c *baseDDD) GetRepo() eh.ReadRepo {
	return c.redfishRepo
}

func (c *baseDDD) GetEventWaiter() *utils.EventWaiter {
	return c.waiter
}

func FindUser(ctx context.Context, s DDDFunctions, user string) (account *RedfishResource, err error) {
	// start looking up user in auth service
	tree, err := GetTree(ctx, s.GetRepo(), s.GetTreeID())
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

func GetPrivileges(ctx context.Context, s DDDFunctions, account *RedfishResource) (privileges []string) {
	// start looking up user in auth service
	tree, err := GetTree(ctx, s.GetRepo(), s.GetTreeID())
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
