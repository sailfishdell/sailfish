package domain

import (
	"context"
	"errors"
	"fmt"
	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/utils"

	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	eventbus "github.com/superchalupa/eventhorizon/eventbus/local"
	eventstore "github.com/superchalupa/eventhorizon/eventstore/memory"
	eventpublisher "github.com/superchalupa/eventhorizon/publisher/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"
)

var _ = fmt.Println

type DDDFunctions interface {
	MakeFullyQualifiedV1(string) string
	GetBaseURI() string

	GetTreeID() eh.UUID

	GetEventStore() eh.EventStore
	GetEventBus() eh.EventBus
	GetEventHandler() eh.EventHandler
	GetEventPublisher() eh.EventPublisher
	GetEventWaiter() EventWaiter

	GetCommandBus() eh.CommandBus
	GetReadRepo() eh.ReadRepo
	GetReadWriteRepo() eh.ReadWriteRepo
}

type EventWaiter interface {
	SetupWait(match func(eh.Event) bool) (id eh.UUID, ch chan eh.Event)
	CancelWait(id eh.UUID)
	Notify(context.Context, eh.Event) error
}

type baseDDD struct {
	baseURI string
	verURI  string

	treeID eh.UUID

	eventStore     eh.EventStore
	eventBus       eh.EventBus
	eventPublisher eh.EventPublisher
	waiter         EventWaiter

	cmdbus      eh.CommandBus
	redfishRepo eh.ReadWriteRepo
}

func BaseDDDFactory(baseURI, verURI string, f ...interface{}) DDDFunctions {
	b := &baseDDD{
		baseURI: baseURI,
		verURI:  verURI,
	}

	if b.eventStore == nil {
		b.eventStore = eventstore.NewEventStore()
	}

	if b.eventBus == nil {
		b.eventBus = eventbus.NewEventBus()
		//eventBus.SetHandlingStrategy( eh.AsyncEventHandlingStrategy )
	}

	if b.eventPublisher == nil {
		b.eventPublisher = eventpublisher.NewEventPublisher()
		//eventPublisher.SetHandlingStrategy( eh.AsyncEventHandlingStrategy )
		b.eventBus.SetPublisher(b.eventPublisher)
	}

	if b.cmdbus == nil {
		b.cmdbus = commandbus.NewCommandBus()
	}

	if b.redfishRepo == nil {
		b.redfishRepo = repo.NewRepo()
	}

	b.treeID = eh.NewUUID()

	if b.waiter == nil {
		b.waiter = utils.NewEventWaiter()
		b.eventPublisher.AddObserver(b.waiter)
	}

	return b
}

func (c *baseDDD) GetEventStore() eh.EventStore {
	return c.eventStore
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

func (c *baseDDD) GetEventBus() eh.EventBus {
	return c.eventBus
}

func (c *baseDDD) GetEventHandler() eh.EventHandler {
	return c.eventBus.(eh.EventHandler)
}

func (c *baseDDD) GetReadRepo() eh.ReadRepo {
	return c.redfishRepo
}

func (c *baseDDD) GetReadWriteRepo() eh.ReadWriteRepo {
	return c.redfishRepo
}

func (c *baseDDD) GetEventWaiter() EventWaiter {
	return c.waiter
}

func (c *baseDDD) GetEventPublisher() eh.EventPublisher {
	return c.eventPublisher
}

func FindUser(ctx context.Context, s DDDFunctions, user string) (account *RedfishResource, err error) {
	// start looking up user in auth service
	tree, err := GetTree(ctx, s.GetReadRepo(), s.GetTreeID())
	if err != nil {
		return nil, errors.New("Malformed tree")
	}

	// get the root service reference
	rootService, err := tree.GetRedfishResourceFromTree(ctx, s.GetReadRepo(), s.MakeFullyQualifiedV1(""))
	if err != nil {
		return nil, errors.New("Malformed tree root resource")
	}

	// Pull up the Accounts Collection
	accounts, err := tree.WalkRedfishResourceTree(ctx, s.GetReadRepo(), rootService, "AccountService", "@odata.id", "Accounts", "@odata.id")
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
		a, _ := tree.GetRedfishResourceFromTree(ctx, s.GetReadRepo(), m["@odata.id"].(string))
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
	tree, err := GetTree(ctx, s.GetReadRepo(), s.GetTreeID())
	if err != nil {
		return
	}

	role, _ := tree.WalkRedfishResourceTree(ctx, s.GetReadRepo(), account, "Links", "Role", "@odata.id")
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
