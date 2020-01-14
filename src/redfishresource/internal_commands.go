package domain

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &UpdateMetricRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateRedfishResourceProperties{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateRedfishResourceProperties2{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResourceProperty{} })
}

const (
	CreateRedfishResourceCommand                 = eh.CommandType("internal:RedfishResource:Create")
	RemoveRedfishResourceCommand                 = eh.CommandType("internal:RedfishResource:Remove")
	UpdateMetricRedfishResourcePropertiesCommand = eh.CommandType("internal:RedfishResourceProperties:UpdateMetric")
	UpdateRedfishResourcePropertiesCommand       = eh.CommandType("internal:RedfishResourceProperties:Update")
	UpdateRedfishResourcePropertiesCommand2      = eh.CommandType("internal:RedfishResourceProperties:Update:2")
	RemoveRedfishResourcePropertyCommand         = eh.CommandType("internal:RedfishResourceProperties:Remove")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&CreateRedfishResource{})
var _ = eh.Command(&RemoveRedfishResource{})
var _ = eh.Command(&UpdateRedfishResourceProperties{})
var _ = eh.Command(&UpdateRedfishResourceProperties2{})
var _ = eh.Command(&RemoveRedfishResourceProperty{})

var immutableProperties = []string{"@odata.id", "@odata.type", "@odata.context"}

// CreateRedfishResource Command
type CreateRedfishResource struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string
	Type        string
	Context     string
	Privileges  map[string]interface{}

	// optional stuff
	Headers       map[string]string      `eh:"optional"`
	Plugin        string                 `eh:"optional"`
	DefaultFilter string                 `eh:"optional"`
	Properties    map[string]interface{} `eh:"optional"`
	Meta          map[string]interface{} `eh:"optional"`
	Private       map[string]interface{} `eh:"optional"`
}

// ShoudlSave satisfies the ShouldSaver interface to tell CommandHandler to save this to DB
func (c *CreateRedfishResource) ShouldSave() bool { return true }

// AggregateType satisfies base Aggregate interface
func (c *CreateRedfishResource) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *CreateRedfishResource) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *CreateRedfishResource) CommandType() eh.CommandType { return CreateRedfishResourceCommand }

func (c *CreateRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {

	requestLogger := log.ContextLogger(ctx, "internal_commands")
	requestLogger.Info("CreateRedfishResource", "META", a.Properties.Meta)

	if a.ID != eh.UUID("") {
		requestLogger.Error("Aggregate already exists!", "command", "CreateRedfishResource", "UUID", a.ID, "URI", a.ResourceURI, "request_URI", c.ResourceURI)
		return errors.New("already created")
	}
	a.ID = c.ID
	a.ResourceURI = c.ResourceURI
	a.DefaultFilter = c.DefaultFilter
	a.Plugin = c.Plugin
	a.Headers = make(map[string]string, len(c.Headers))
	for k, v := range c.Headers {
		a.Headers[k] = v
	}

	a.PrivilegeMap = make(map[HTTPReqType]interface{}, len(c.Privileges))
	for k, v := range c.Privileges {
		a.PrivilegeMap[MapStringToHTTPReq(k)] = v
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

	v := map[string]interface{}{}
	a.Properties.Value = v
	a.Properties.Parse(c.Properties)
	a.Properties.Meta = c.Meta

	var resourceURI []string
	// preserve slashes
	for _, x := range strings.Split(c.ResourceURI, "/") {
		resourceURI = append(resourceURI, url.PathEscape(x))
	}

	v["@odata.id"] = strings.Join(resourceURI, "/")
	v["@odata.type"] = c.Type
	v["@odata.context"] = c.Context

	// send out event that it's created first
	a.PublishEvent(eh.NewEvent(RedfishResourceCreated, &RedfishResourceCreatedData{
		ID:          c.ID,
		ResourceURI: c.ResourceURI,
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
	ResourceURI string  `eh:"optional"`
}

// ShoudlSave satisfies the ShouldSaver interface to tell CommandHandler to save this to DB
func (c *RemoveRedfishResource) ShouldSave() bool { return true }

// AggregateType satisfies base Aggregate interface
func (c *RemoveRedfishResource) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *RemoveRedfishResource) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *RemoveRedfishResource) CommandType() eh.CommandType { return RemoveRedfishResourceCommand }

func (c *RemoveRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.PublishEvent(eh.NewEvent(RedfishResourceRemoved, &RedfishResourceRemovedData{
		ID:          c.ID,
		ResourceURI: a.ResourceURI,
	}, time.Now()))
	return nil
}

type RemoveRedfishResourceProperty struct {
	ID       eh.UUID `json:"id"`
	Property string  `eh:"optional"`
}

// ShoudlSave satisfies the ShouldSaver interface to tell CommandHandler to save this to DB
func (c *RemoveRedfishResourceProperty) ShouldSave() bool { return true }

// AggregateType satisfies base Aggregate interface
func (c *RemoveRedfishResourceProperty) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *RemoveRedfishResourceProperty) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *RemoveRedfishResourceProperty) CommandType() eh.CommandType {
	return RemoveRedfishResourcePropertyCommand
}
func (c *RemoveRedfishResourceProperty) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	properties := a.Properties.Value.(map[string]interface{})
	for key := range properties {
		if key == c.Property {
			delete(properties, key)
		}
	}
	return nil
}

// toUpdate	{path2key : value}
type UpdateRedfishResourceProperties2 struct {
	ID         eh.UUID `json:"id"`
	Properties map[string]interface{}
}

// ShoudlSave satisfies the ShouldSaver interface to tell CommandHandler to save this to DB
func (c *UpdateRedfishResourceProperties2) ShouldSave() bool { return true }

// AggregateType satisfies base Aggregate interface
func (c *UpdateRedfishResourceProperties2) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *UpdateRedfishResourceProperties2) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *UpdateRedfishResourceProperties2) CommandType() eh.CommandType {
	return UpdateRedfishResourcePropertiesCommand2
}

// aggregate is a.Properties.(RedfishresourceProperty)
// going through the aggregate it is [map]*RedfishResourceProperty...
// Updated to append to list.  TODO need a way to clean lists and prevent duplicates
func UpdateAgg(a *RedfishResourceAggregate, pathSlice []string, v interface{}, appendLimit int) error {
	loc, ok := a.Properties.Value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("updateagg: property is wrong type %T", a.Properties.Value)
	}

	plen := len(pathSlice) - 1
	for i, p := range pathSlice {
		k, ok := loc[p]
		if !ok {
			loc[p] = &RedfishResourceProperty{}
			k = loc[p]
		}
		switch k.(type) {
		case *RedfishResourceProperty:
			k2, ok := k.(*RedfishResourceProperty)
			if !ok {
				return errors.New("updateAgg Failed, RedfishResourcePropertyFailed")
			}
			// metric events have the data appended
			switch v.(type) {
			case []interface{}, []map[string]interface{}:
				j := k2.Value.([]interface{})
				aggSLen := len(j)
				v2 := v.([]interface{})
				if aggSLen >= appendLimit {
					continue
				}
				if appendLimit < aggSLen+len(v2) {
					k2.Parse(v2[appendLimit-aggSLen:])
				} else {
					k2.Parse(v2)
				}
				k2.Parse(j)
				return nil
			default:
				if plen != i {
					if k2.Value == nil {
						loc = map[string]interface{}{}
						k2.Value = loc
						continue
					} else {
						tmp := k2.Value
						loc, ok = tmp.(map[string]interface{})
					}

				} else if (plen == i) && (k2.Value != v) {
					k2.Value = v
				} else if plen == i {
					return nil
				}
			}

		default:
			return fmt.Errorf("agg update for slice %+v, received type %T instead of *RedfishResourceProperty", pathSlice, k)
		}
	}
	return nil

}

func GetValueinAgg(a *RedfishResourceAggregate, pathSlice []string) interface{} {
	a.Properties.Lock()
	defer a.Properties.Unlock()
	loc, ok := a.Properties.Value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("getValueinAgg: aggregate value is not a map[string]interface{}, but %T", a.Properties.Value)
	}

	plen := len(pathSlice) - 1
	for i, p := range pathSlice {
		k, ok := loc[p]
		if !ok {
			return fmt.Errorf("UpdateAgg Failed can not find %s in %+v", p, loc)
		}
		switch k.(type) {
		case *RedfishResourceProperty:
			k2, ok := k.(*RedfishResourceProperty)
			if !ok {
				return fmt.Errorf("UpdateAgg Failed, RedfishResourcePropertyFailed")
			}
			// metric events have the data appended
			if plen == i {
				return k2.Value
			} else if plen == i {
				return nil
			} else {
				tmp := k2.Value
				loc, ok = tmp.(map[string]interface{})
				if !ok {
					return fmt.Errorf("UpdateAgg Failed %s type cast to map[string]interface{} for %+v  errored for %+v", a.ResourceURI, p, pathSlice)
				}
			}

		default:
			return fmt.Errorf("agg update for slice %+v, received type %T instead of *RedfishResourceProperty", pathSlice, k)
		}
	}

	return nil

}

//  This is handled by eventhorizon code.
//  When a CommandHandler "Handle" is called it will retrieve the aggregate from the DB.  and call this Handle. Then save the aggregate 'a' back to the db.  no locking is required..
// provide error when no change made..
func (c *UpdateRedfishResourceProperties2) Handle(ctx context.Context, a *RedfishResourceAggregate) error {

	if a.ID == eh.UUID("") {
		requestLogger := log.ContextLogger(ctx, "internal_commands")
		requestLogger.Error("Aggregate does not exist!", "UUID", a.ID, "URI", a.ResourceURI, "COMMAND", c)
		return errors.New("non existent aggregate")
	}

	var err error = nil

	d := &RedfishResourcePropertiesUpdatedData2{
		ID:            c.ID,
		ResourceURI:   a.ResourceURI,
		PropertyNames: make(map[string]interface{}),
	}

	// update properties in aggregate
	for k, v := range c.Properties {
		pathSlice := strings.Split(k, "/")

		err := UpdateAgg(a, pathSlice, v, 0)

		if err == nil {
			d.PropertyNames[k] = v
		}
	}

	if len(d.PropertyNames) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertiesUpdated2, d, time.Now()))
	}
	return err
}

type UpdateMetricRedfishResource struct {
	ID               eh.UUID                `json:"id"`
	Properties       map[string]interface{} `eh:"optional"`
	AppendLimit      int
	ReportUpdateType string
}

// ShoudlSave satisfies the ShouldSaver interface to tell CommandHandler to save this to DB
func (c *UpdateMetricRedfishResource) ShouldSave() bool { return true }

// AggregateType satisfies base Aggregate interface
func (c *UpdateMetricRedfishResource) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *UpdateMetricRedfishResource) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *UpdateMetricRedfishResource) CommandType() eh.CommandType {
	return UpdateMetricRedfishResourcePropertiesCommand
}

//reportUpdateType int // 0-AppendStopsWhenFull, 1-AppendWrapsWhenFull, 3- NewReport, 4-Overwrite

// assume AppendStopsWhenFull
func (c *UpdateMetricRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	for k, v := range c.Properties {
		pathSlice := strings.Split(k, "/")
		if err := UpdateAgg(a, pathSlice, v, int(c.AppendLimit)); err != nil {
			fmt.Println("failed to updated agg")
			return err
		}
	}

	return nil
}

type UpdateRedfishResourceProperties struct {
	ID         eh.UUID                `json:"id"`
	Properties map[string]interface{} `eh:"optional"`
}

// ShoudlSave satisfies the ShouldSaver interface to tell CommandHandler to save this to DB
func (c *UpdateRedfishResourceProperties) ShouldSave() bool { return true }

// AggregateType satisfies base Aggregate interface
func (c *UpdateRedfishResourceProperties) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *UpdateRedfishResourceProperties) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *UpdateRedfishResourceProperties) CommandType() eh.CommandType {
	return UpdateRedfishResourcePropertiesCommand
}
func (c *UpdateRedfishResourceProperties) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
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

	a.Properties.Parse(c.Properties)

	if len(d.PropertyNames) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertiesUpdated, d, time.Now()))
	}
	if len(e.Meta) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertyMetaUpdated, e, time.Now()))
	}

	return nil
}
