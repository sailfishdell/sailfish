package domain

import (
	"context"
	"errors"
	"fmt"
	eh "github.com/looplab/eventhorizon"
	"strings"
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
	a.propertyPluginMu.Lock() // write lock
	defer a.propertyPluginMu.Unlock()
	a.propertyPlugin = map[string]map[string]map[string]interface{}{}

	a.InitProperties()

	a.PrivilegeMap = map[string]interface{}{}
	a.Permissions = map[string]interface{}{}
	a.Headers = map[string]string{}
	a.Private = map[string]interface{}{}

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

	for k, v := range c.Properties {
		// this is sort of a pain: need to get data from a
		// map[string]interface{} into a
		// map[string]map[string]map[string]interface{} by iterating through
		// the whole dang thing and copying with the appropriate type
		// assertions. skip stuff that isn't the correct type.
		if strings.HasSuffix(k, "@meta") {
			// strip off "@meta" so property name matches in both hashes
			name := k[:len(k)-5]
			fmt.Printf("Add to PropertyMap: %s = %s\n", name, v)

			// make sure that the input is structured properly... skip it if not
			_, ok := v.(map[string]interface{})
			if !ok {
				fmt.Printf("\tDidn't type assert correctly...\n")
				continue
			}

			// iterate over all of the map values and cast...
			fmt.Printf("\tcheck1 pass\n")
			for k2, v2 := range v.(map[string]interface{}) {
				_, ok = v2.(map[string]interface{})
				if !ok {
					fmt.Printf("Cleanse incorrect type: %s\n", v2)
					continue
				}

				if _, ok := a.propertyPlugin[name]; !ok {
					a.propertyPlugin[name] = map[string]map[string]interface{}{}
				}
				a.propertyPlugin[name][k2] = v2.(map[string]interface{})

				if _, ok := e.Meta[name]; !ok {
					e.Meta[name] = map[string]interface{}{}
				}
				e.Meta[name].(map[string]interface{})[k2] = v2
			}

			fmt.Printf("\tcheck2 pass: %s\n", v)

		} else {
			// atomically update properties: Add property to notification if
			// it's changing (don't notify if no new property change)
			a.MutateProperty(func(properties map[string]interface{}) {
				if properties[k] != v {
					properties[k] = v
					d.PropertyNames = append(d.PropertyNames, k)
				}
			})
		}
	}

	a.SetProperty("@odata.id", c.ResourceURI)
	a.SetProperty("@odata.type", c.Type)
	a.SetProperty("@odata.context", c.Context)

	// if command claims that this will be a collection, automatically set up the Members property
	if c.Collection {
		a.MutateProperty(func(properties map[string]interface{}) {
			if _, ok := properties["Members"]; !ok {
				// didn't previously exist, create it
				properties["Members"] = []map[string]interface{}{}
			} else {
				switch properties["Members"].(type) {
				case []map[string]interface{}:
				default: // somehow got invalid type here, fix it up
					properties["Members"] = []map[string]interface{}{}
				}
			}
			properties["Members@odata.count"] = len(properties["Members"].([]map[string]interface{}))
		})
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

	for k, v := range c.Properties {
		if strings.HasSuffix(k, "@meta") {
			//if a.PropertyPlugin[k] != v {
			a.propertyPlugin[k] = v.(map[string]map[string]interface{})
			e.Meta[k] = v
			//}
		} else {
			a.MutateProperty(func(properties map[string]interface{}) {
				if properties[k] != v {
					properties[k] = v
					d.PropertyNames = append(d.PropertyNames, k)
				}
			})
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
	// manipulate properties under lock, also update length of members under the same lock to ensure everything is consistent
	a.MutateProperty(func(properties map[string]interface{}) {
		if _, ok := properties["Members"]; !ok {
			// didn't previously exist, create it
			properties["Members"] = []map[string]interface{}{}
		} else {
			switch properties["Members"].(type) {
			case []map[string]interface{}:
			default: // somehow got invalid type here, fix it up
				properties["Members"] = []map[string]interface{}{}
			}
		}
		properties["Members"] = append(properties["Members"].([]map[string]interface{}), map[string]interface{}{"@odata.id": c.ResourceURI})
		properties["Members@odata.count"] = len(properties["Members"].([]map[string]interface{}))
	})

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

	// manipulate properties under lock, also update length of members under the same lock to ensure everything is consistent
	a.MutateProperty(func(properties map[string]interface{}) {
		if _, ok := properties["Members"]; !ok {
			// didn't previously exist, create it
			properties["Members"] = []map[string]interface{}{}
		} else {
			switch properties["Members"].(type) {
			case []map[string]interface{}:
			default: // somehow got invalid type here, fix it up
				properties["Members"] = []map[string]interface{}{}
			}
		}

		if collection, ok := properties["Members"]; ok {
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
				s = s[:len(s)-numToSlice]
			}
		}

		properties["Members@odata.count"] = len(properties["Members"].([]map[string]interface{}))
	})

	return nil
}
