package fans

import (
	"context"
	"fmt"

	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const (
	FansPlugin = domain.PluginType("fans")
)

type RedfishFan struct {
	OdataID         string `json:"@odata.id"`
	MemberID        string
	Name            string
	PhysicalContext string
	Reading         float64
	ReadingUnits    string

	UpperThresholdNonCritical float64
	UpperThresholdCritical    float64
	UpperThresholdFatal       float64

	LowerThresholdNonCritical float64
	LowerThresholdCritical    float64
	LowerThresholdFatal       float64

	MinReadingRange float64
	MaxReadingRange float64
}

func (s *RedfishFan) SetOdataID(id string) {
	s.OdataID = id
}

func (s *RedfishFan) SetMemberID(id string) {
	s.MemberID = id
}

type sensorInt interface {
	SetOdataID(string)
	SetMemberID(string)
}

type odataInt interface {
	GetOdataID() string
	GetUUID() eh.UUID
}

type service struct {
	*plugins.Service
	therm   odataInt
	sensors map[string]sensorInt
}

func New(options ...interface{}) (*service, error) {
	p := &service{
		Service: plugins.NewService(plugins.PluginType(FansPlugin)),
		sensors: map[string]sensorInt{},
	}
	p.ApplyOption(options...)
	return p, nil
}

func InThermal(b odataInt) Option {
	return func(p *service) error {
		p.therm = b
		return nil
	}
}

func WithSensor(name string, sensor sensorInt) Option {
	return func(s *service) error {
		s.Lock()
		defer s.Unlock()
		s.sensors[name] = sensor
		return nil
	}
}

func (s *service) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	s.Lock()
	defer s.Unlock()

	res := []sensorInt{}
	var idx = 0
	for _, v := range s.sensors {
		// make a copy so we can move on with life after we return (reduce locking issues)
		// TODO: does this actually work?
		var s sensorInt = v
		s.SetOdataID(fmt.Sprintf("%s/%s/%d", agg.ResourceURI, "#/Fans", idx))
		s.SetMemberID(fmt.Sprintf("%d", idx))
		res = append(res, s)
		idx++
	}
	rrp.Value = res
}

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: s.therm.GetUUID(),
			Properties: map[string]interface{}{
				"Fans@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType())}},
			},
		})
}
