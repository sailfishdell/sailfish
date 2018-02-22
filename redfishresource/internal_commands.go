package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateRedfishResourceProperties{} })
	eh.RegisterCommand(func() eh.Command { return &AddResourceToRedfishResourceCollection{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveResourceFromRedfishResourceCollection{} })
}

const (
	CreateRedfishResourceCommand                       = eh.CommandType("internal:RedfishResource:Create")
	RemoveRedfishResourceCommand                       = eh.CommandType("internal:RedfishResource:Remove")
	UpdateRedfishResourcePropertiesCommand             = eh.CommandType("internal:RedfishResourceProperties:Update")
	AddResourceToRedfishResourceCollectionCommand      = eh.CommandType("internal:RedfishResourceCollection:Add")
	RemoveResourceFromRedfishResourceCollectionCommand = eh.CommandType("internal:RedfishResourceCollection:Remove")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&CreateRedfishResource{})
var _ = eh.Command(&RemoveRedfishResource{})
var _ = eh.Command(&UpdateRedfishResourceProperties{})
var _ = eh.Command(&AddResourceToRedfishResourceCollection{})
var _ = eh.Command(&RemoveResourceFromRedfishResourceCollection{})

var immutableProperties = []string{"@odata.id", "@odata.type", "@odata.context"}

// CreateRedfishResource Command
type CreateRedfishResource struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string
	Type        string
	Context     string
	Privileges  map[string]interface{}

	// optional stuff
	Plugin     string                 `eh:"optional"`
	Properties map[string]interface{} `eh:"optional"`
	Meta       map[string]interface{} `eh:"optional"`
	Private    map[string]interface{} `eh:"optional"`
	Collection bool                   `eh:"optional"`
}

func (c *CreateRedfishResource) AggregateType() eh.AggregateType { return AggregateType }
func (c *CreateRedfishResource) AggregateID() eh.UUID            { return c.ID }
func (c *CreateRedfishResource) CommandType() eh.CommandType     { return CreateRedfishResourceCommand }

func (c *CreateRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	if a.ID != eh.UUID("") {
		fmt.Printf("CREATE COMMAND: Aggregate already exists!\n")
		return errors.New("Already created!")
	}
	a.ID = c.ID
	a.ResourceURI = c.ResourceURI
	a.Plugin = c.Plugin
	if a.Plugin == "" {
		a.Plugin = "RedfishResource"
	}
	a.PrivilegeMap = map[string]interface{}{}
	a.Headers = map[string]string{}

	for k, v := range c.Privileges {
		a.PrivilegeMap[k] = v
	}

	// ensure no collisions
	for _, p := range immutableProperties {
		delete(c.Properties, p)
	}

	d := RedfishResourcePropertiesUpdatedData{
		ID:            c.ID,
		ResourceURI:   a.ResourceURI,
		PropertyNames: []string{},
	}
	e := RedfishResourcePropertyMetaUpdatedData{
		ID:          c.ID,
		ResourceURI: a.ResourceURI,
		Meta:        map[string]interface{}{},
	}

	a.propertiesMu.Lock()
	a.properties.Value = map[string]interface{}{}
	a.properties.Parse(c.Properties)
	a.properties.Meta = c.Meta
	fmt.Printf("GOT META: %s\n", a.properties.Meta)
	a.propertiesMu.Unlock()

	a.SetProperty("@odata.id", c.ResourceURI)
	a.SetProperty("@odata.type", c.Type)
	a.SetProperty("@odata.context", c.Context)

	// if command claims that this will be a collection, automatically set up the Members property
	if c.Collection {
		a.EnsureCollection()
	} else {
		a.DeleteProperty("Members")
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		// send out event that it's created first
		a.eventBus.HandleEvent(ctx, eh.NewEvent(RedfishResourceCreated, RedfishResourceCreatedData{
			ID:          c.ID,
			ResourceURI: c.ResourceURI,
			Collection:  c.Collection,
		}, time.Now()))

		// then send out possible notifications about changes in the properties or meta
		if len(d.PropertyNames) > 0 {
			a.eventBus.HandleEvent(ctx, eh.NewEvent(RedfishResourcePropertiesUpdated, d, time.Now()))
		}
		if len(e.Meta) > 0 {
			a.eventBus.HandleEvent(ctx, eh.NewEvent(RedfishResourcePropertyMetaUpdated, e, time.Now()))
		}
	}()

	return nil
}

// RemoveRedfishResource Command
type RemoveRedfishResource struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string
}

func (c *RemoveRedfishResource) AggregateType() eh.AggregateType { return AggregateType }
func (c *RemoveRedfishResource) AggregateID() eh.UUID            { return c.ID }
func (c *RemoveRedfishResource) CommandType() eh.CommandType     { return RemoveRedfishResourceCommand }

func (c *RemoveRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.eventBus.HandleEvent(ctx, eh.NewEvent(RedfishResourceRemoved, RedfishResourceRemovedData{
		ID:          c.ID,
		ResourceURI: c.ResourceURI,
	}, time.Now()))
	return nil
}

type UpdateRedfishResourceProperties struct {
	ID         eh.UUID                `json:"id"`
	Properties map[string]interface{} `eh:"optional"`
}

func (c *UpdateRedfishResourceProperties) AggregateType() eh.AggregateType { return AggregateType }
func (c *UpdateRedfishResourceProperties) AggregateID() eh.UUID            { return c.ID }
func (c *UpdateRedfishResourceProperties) CommandType() eh.CommandType {
	return UpdateRedfishResourcePropertiesCommand
}
func (c *UpdateRedfishResourceProperties) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// TODO: Need to send property updated on update

	// ensure no collisions with immutable properties
	for _, p := range immutableProperties {
		delete(c.Properties, p)
	}

	d := RedfishResourcePropertiesUpdatedData{
		ID:            c.ID,
		ResourceURI:   a.ResourceURI,
		PropertyNames: []string{},
	}
	e := RedfishResourcePropertyMetaUpdatedData{
		ID:          c.ID,
		ResourceURI: a.ResourceURI,
		Meta:        map[string]interface{}{},
	}

	a.propertiesMu.Lock()
	a.properties.Parse(c.Properties)
	a.propertiesMu.Unlock()

	if len(d.PropertyNames) > 0 {
		a.eventBus.HandleEvent(ctx, eh.NewEvent(RedfishResourcePropertiesUpdated, d, time.Now()))
	}
	if len(e.Meta) > 0 {
		a.eventBus.HandleEvent(ctx, eh.NewEvent(RedfishResourcePropertyMetaUpdated, e, time.Now()))
	}

	return nil
}

type AddResourceToRedfishResourceCollection struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string  // resource to add to the collection
}

func (c *AddResourceToRedfishResourceCollection) AggregateType() eh.AggregateType {
	return AggregateType
}
func (c *AddResourceToRedfishResourceCollection) AggregateID() eh.UUID { return c.ID }
func (c *AddResourceToRedfishResourceCollection) CommandType() eh.CommandType {
	return AddResourceToRedfishResourceCollectionCommand
}
func (c *AddResourceToRedfishResourceCollection) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.AddCollectionMember(c.ResourceURI)
	return nil
}

type RemoveResourceFromRedfishResourceCollection struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string
}

func (c *RemoveResourceFromRedfishResourceCollection) AggregateType() eh.AggregateType {
	return AggregateType
}
func (c *RemoveResourceFromRedfishResourceCollection) AggregateID() eh.UUID { return c.ID }
func (c *RemoveResourceFromRedfishResourceCollection) CommandType() eh.CommandType {
	return RemoveResourceFromRedfishResourceCollectionCommand
}
func (c *RemoveResourceFromRedfishResourceCollection) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.RemoveCollectionMember(c.ResourceURI)
	return nil
}
