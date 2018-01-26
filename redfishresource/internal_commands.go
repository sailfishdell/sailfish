package domain

import (
	"context"
	"errors"
	"fmt"
    "strings"
	eh "github.com/looplab/eventhorizon"
	"time"
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
	Private    map[string]interface{} `eh:"optional"`
	Collection bool                   `eh:"optional"`
}

func (c *CreateRedfishResource) AggregateType() eh.AggregateType { return AggregateType }
func (c *CreateRedfishResource) AggregateID() eh.UUID            { return c.ID }
func (c *CreateRedfishResource) CommandType() eh.CommandType     { return CreateRedfishResourceCommand }

func (c *CreateRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("CreateRedfishResource (COMMAND)\n")
	if a.ID != eh.UUID("") {
		fmt.Printf("Aggregate already exists!\n")
		return errors.New("Already created!")
	}
	a.ID = c.ID
	a.ResourceURI = c.ResourceURI
	a.Plugin = c.Plugin
	if a.Plugin == "" {
		a.Plugin = "RedfishResource"
	}
	a.PropertyPlugin = map[string]interface{}{}
	a.Properties = map[string]interface{}{}
	a.PrivilegeMap = map[string]interface{}{}
	a.Permissions = map[string]interface{}{}
	a.Headers = map[string]string{}
	a.Private = map[string]interface{}{}

	for k, v := range c.Privileges {
		a.PrivilegeMap[k] = v
	}

    // ensure no collisions
    delete(c.Properties, "@odata.id")
    delete(c.Properties, "@odata.type")
    delete(c.Properties, "@odata.context")

    d := RedfishResourcePropertiesUpdatedData{
                ID:          c.ID,
                ResourceURI: a.ResourceURI,
                PropertyNames: []string{},
            }
    e := RedfishResourcePropertyMetaUpdatedData{
                ID:          c.ID,
                ResourceURI: a.ResourceURI,
                Meta: map[string]interface{}{},
                }

	for k, v := range c.Properties {
        if strings.HasSuffix(k, "@meta") {
            if a.PropertyPlugin[k] != v {
                a.PropertyPlugin[k] = v
                e.Meta[k] = v
            }
        } else {
            if a.Properties[k] != v {
                a.Properties[k] = v
                d.PropertyNames = append(d.PropertyNames, k)
            }
        }
	}

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

	a.Properties["@odata.id"] = c.ResourceURI
	a.Properties["@odata.type"] = c.Type
	a.Properties["@odata.context"] = c.Context

	// if command claims that this will be a collection, automatically set up the Members property
	if c.Collection {
		if _, ok := a.Properties["Members"]; !ok {
            // didn't previously exist, create it
			a.Properties["Members"] = []map[string]interface{}{}
		} else {
			switch a.Properties["Members"].(type) {
			case []map[string]interface{}:
			default: // somehow got invalid type here, fix it up
				a.Properties["Members"] = []map[string]interface{}{}
			}
		}
		a.Properties["Members@odata.count"] = len(a.Properties["Members"].([]map[string]interface{}))
	}
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
    // TODO: Filter out immutable properties: type, context, id...
    // TODO: support JSON Pointer or similar format to do a better/more granular update of properties

    d := RedfishResourcePropertiesUpdatedData{
                ID:          c.ID,
                ResourceURI: a.ResourceURI,
                PropertyNames: []string{},
            }
    e := RedfishResourcePropertyMetaUpdatedData{
                ID:          c.ID,
                ResourceURI: a.ResourceURI,
                Meta: map[string]interface{}{},
                }

	for k, v := range c.Properties {
        if strings.HasSuffix(k, "@meta") {
            if a.PropertyPlugin[k] != v {
                a.PropertyPlugin[k] = v
                e.Meta[k] = v
            }
        } else {
            if a.Properties[k] != v {
                a.Properties[k] = v
                d.PropertyNames = append(d.PropertyNames, k)
            }
        }
	}

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
	// TODO: send property updated event
	if collection, ok := a.Properties["Members"]; ok {
		if co, ok := collection.([]map[string]interface{}); ok {
			a.Properties["Members"] = append(co, map[string]interface{}{"@odata.id": c.ResourceURI})
			a.Properties["Members@odata.count"] = len(a.Properties["Members"].([]map[string]interface{}))
		}
	}
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
	// TODO: send property updated event
	if collection, ok := a.Properties["Members"]; ok {
		numToSlice := 0
		if s, ok := collection.([]map[string]interface{}); ok {
			for i, v := range s {
				if v["@odata.id"] == c.ResourceURI {
					// move the ones to be removed to the end
					numToSlice = numToSlice + 1
					s[len(s)-numToSlice], s[i] = s[i], s[len(s)-numToSlice]
					break
				}
			}
			// and then slice off the end
			a.Properties["Members"] = s[:len(s)-numToSlice]
		}
		a.Properties["Members@odata.count"] = len(a.Properties["Members"].([]map[string]interface{}))
	}

	return nil
}
