package domain

import (
	"context"
	"errors"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/mitchellh/mapstructure"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateRedfishResourceProperties{} })
	eh.RegisterCommand(func() eh.Command { return &AddResourceToRedfishResourceCollection{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveResourceFromRedfishResourceCollection{} })
	eh.RegisterCommand(func() eh.Command { return &InjectEvent{} })
}

const (
	CreateRedfishResourceCommand                       = eh.CommandType("internal:RedfishResource:Create")
	RemoveRedfishResourceCommand                       = eh.CommandType("internal:RedfishResource:Remove")
	UpdateRedfishResourcePropertiesCommand             = eh.CommandType("internal:RedfishResourceProperties:Update")
	AddResourceToRedfishResourceCollectionCommand      = eh.CommandType("internal:RedfishResourceCollection:Add")
	RemoveResourceFromRedfishResourceCollectionCommand = eh.CommandType("internal:RedfishResourceCollection:Remove")
	InjectEventCommand                                 = eh.CommandType("internal:Event:Inject")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&CreateRedfishResource{})
var _ = eh.Command(&RemoveRedfishResource{})
var _ = eh.Command(&UpdateRedfishResourceProperties{})
var _ = eh.Command(&AddResourceToRedfishResourceCollection{})
var _ = eh.Command(&RemoveResourceFromRedfishResourceCollection{})
var _ = eh.Command(&InjectEvent{})

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

// AggregateType satisfies base Aggregate interface
func (c *CreateRedfishResource) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *CreateRedfishResource) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *CreateRedfishResource) CommandType() eh.CommandType { return CreateRedfishResourceCommand }

func (c *CreateRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	requestLogger := ContextLogger(ctx, "internal_commands")
	requestLogger.Info("CreateRedfishResource", "META", a.Properties.Meta)

	if a.ID != eh.UUID("") {
		requestLogger.Error("Aggregate already exists!", "command", "CreateRedfishResource", "UUID", a.ID, "URI", a.ResourceURI, "request_URI", c.ResourceURI)
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

	d := &RedfishResourcePropertiesUpdatedData{
		ID:            c.ID,
		ResourceURI:   a.ResourceURI,
		PropertyNames: []string{},
	}
	e := &RedfishResourcePropertyMetaUpdatedData{
		ID:          c.ID,
		ResourceURI: a.ResourceURI,
		Meta:        map[string]interface{}{},
	}

	a.PropertiesMu.Lock()
	v := map[string]interface{}{}
	a.Properties.Value = v
	a.Properties.Parse(c.Properties)
	a.Properties.Meta = c.Meta

	v["@odata.id"] = &RedfishResourceProperty{Value: c.ResourceURI}
	v["@odata.type"] = &RedfishResourceProperty{Value: c.Type}
	v["@odata.context"] = &RedfishResourceProperty{Value: c.Context}

	a.PropertiesMu.Unlock()

	// if command claims that this will be a collection, automatically set up the Members property
	if c.Collection {
		a.EnsureCollection()
	}

	// send out event that it's created first
	a.PublishEvent(eh.NewEvent(RedfishResourceCreated, &RedfishResourceCreatedData{
		ID:          c.ID,
		ResourceURI: c.ResourceURI,
		Collection:  c.Collection,
	}, time.Now()))

	// then send out possible notifications about changes in the properties or meta
	if len(d.PropertyNames) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertiesUpdated, d, time.Now()))
	}
	if len(e.Meta) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertyMetaUpdated, e, time.Now()))
	}

	return nil
}

// RemoveRedfishResource Command
type RemoveRedfishResource struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string
}

// AggregateType satisfies base Aggregate interface
func (c *RemoveRedfishResource) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *RemoveRedfishResource) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *RemoveRedfishResource) CommandType() eh.CommandType { return RemoveRedfishResourceCommand }

func (c *RemoveRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.PublishEvent(eh.NewEvent(RedfishResourceRemoved, &RedfishResourceRemovedData{
		ID:          c.ID,
		ResourceURI: c.ResourceURI,
	}, time.Now()))
	return nil
}

type UpdateRedfishResourceProperties struct {
	ID         eh.UUID                `json:"id"`
	Properties map[string]interface{} `eh:"optional"`
}

// AggregateType satisfies base Aggregate interface
func (c *UpdateRedfishResourceProperties) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *UpdateRedfishResourceProperties) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *UpdateRedfishResourceProperties) CommandType() eh.CommandType {
	return UpdateRedfishResourcePropertiesCommand
}
func (c *UpdateRedfishResourceProperties) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// TODO: Need to send property updated on update

	// ensure no collisions with immutable properties
	for _, p := range immutableProperties {
		delete(c.Properties, p)
	}

	d := &RedfishResourcePropertiesUpdatedData{
		ID:            c.ID,
		ResourceURI:   a.ResourceURI,
		PropertyNames: []string{},
	}
	e := &RedfishResourcePropertyMetaUpdatedData{
		ID:          c.ID,
		ResourceURI: a.ResourceURI,
		Meta:        map[string]interface{}{},
	}

	a.PropertiesMu.Lock()
	a.Properties.Parse(c.Properties)
	a.PropertiesMu.Unlock()

	if len(d.PropertyNames) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertiesUpdated, d, time.Now()))
	}
	if len(e.Meta) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertyMetaUpdated, e, time.Now()))
	}

	return nil
}

type AddResourceToRedfishResourceCollection struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string  // resource to add to the collection
}

// AggregateType satisfies base Aggregate interface
func (c *AddResourceToRedfishResourceCollection) AggregateType() eh.AggregateType {
	return AggregateType
}

// AggregateID satisfies base Aggregate interface
func (c *AddResourceToRedfishResourceCollection) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
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

// AggregateType satisfies base Aggregate interface
func (c *RemoveResourceFromRedfishResourceCollection) AggregateType() eh.AggregateType {
	return AggregateType
}

// AggregateID satisfies base Aggregate interface
func (c *RemoveResourceFromRedfishResourceCollection) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *RemoveResourceFromRedfishResourceCollection) CommandType() eh.CommandType {
	return RemoveResourceFromRedfishResourceCollectionCommand
}
func (c *RemoveResourceFromRedfishResourceCollection) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.RemoveCollectionMember(c.ResourceURI)
	return nil
}

// gross layering violation, but to avoid import cycles, moved the events here for now
type AttributeArrayUpdatedData struct {
	Attributes []AttributeUpdatedData
}

type AttributeUpdatedData struct {
	ReqID eh.UUID
	FQDD  string
	Group string
	Index string
	Name  string
	Value interface{}
	Error string
}

type InjectEvent struct {
	ID         eh.UUID                  `json:"id" eh:"optional"`
	Name       eh.EventType             `json:"name"`
	EventData  map[string]interface{}   `json:"data" eh:"optional"`
	EventArray []map[string]interface{} `json:"event_array" eh:"optional"`
}

// AggregateType satisfies base Aggregate interface
func (c *InjectEvent) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *InjectEvent) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *InjectEvent) CommandType() eh.CommandType {
	return InjectEventCommand
}

var injectChan chan eh.Event

func StartInjectService(eb eh.EventBus) {
	injectChan = make(chan eh.Event, 1000)
	go func() {
		for {
			event := <-injectChan
			eb.PublishEvent(context.Background(), event) // in a goroutine (comment for grep purposes)
		}
	}()
}

func (c *InjectEvent) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	requestLogger := ContextLogger(ctx, "internal_commands").New("module", "inject_event")

	a.ID = eh.UUID("49467bb4-5c1f-473b-0000-00000000000f")

	eventList := []map[string]interface{}{}
	eventList = append(eventList, c.EventData)
	eventList = append(eventList, c.EventArray...)

	requestLogger.Debug("InjectEvent - NEW ARRAY INJECT")

	trainload := []eh.EventData{}
	for _, eventData := range eventList {
		data, err := eh.CreateEventData(c.Name)
		if err != nil {
			requestLogger.Info("InjectEvent - event type not registered: injecting raw event.", "event name", c.Name, "error", err)
			trainload = append(trainload, eventData)
			continue
		}

		err = mapstructure.Decode(eventData, &data)
		if err != nil {
			requestLogger.Warn("InjectEvent - could not decode event data, skipping event", "error", err, "raw-eventdata", eventData, "dest-event", data)
			trainload = append(trainload, eventData)
			continue
		}

		trainload = append(trainload, data)
		requestLogger.Debug("InjectEvent - publishing", "event name", c.Name, "event_data", data)
	}
	injectChan <- eh.NewEvent(c.Name, trainload, time.Now())

	return nil
}
