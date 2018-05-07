package temperatures

import (
	"context"
	"fmt"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const (
	TemperaturesPlugin = domain.PluginType("temperatures")
)

type RedfishThermalSensor struct {
	OdataID      string `json:"@odata.id"`
	MemberID     string
	Name         string
	SensorNumber int
	// Status                    StdStatus
	ReadingCelsius            float64
	UpperThresholdNonCritical float64
	UpperThresholdCritical    float64
	UpperThresholdFatal       float64
	MinReadingRangeTemp       float64
	MaxReadingRangeTemp       float64
	PhysicalContext           string
}

func (s *RedfishThermalSensor) SetOdataID(id string) {
	s.OdataID = id
}

func (s *RedfishThermalSensor) SetMemberID(id string) {
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
		Service: plugins.NewService(plugins.PluginType(TemperaturesPlugin)),
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
		s.sensors[name] = sensor
		return nil
	}
}

func (s *service) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) {
	s.Lock()
	defer s.Unlock()

	res := []sensorInt{}
	var idx = 0
	for _, v := range s.sensors {
		// make a copy so we can move on with life after we return (reduce locking issues)
		// TODO: does this actually work?
		var s sensorInt = v
		s.SetOdataID(fmt.Sprintf("%s/%s/%d", agg.ResourceURI, "#/Temperatures", idx))
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
				"Temperatures@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType())}},
			},
		})
}
