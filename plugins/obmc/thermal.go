package obmc

import (
	"context"
	"sync"

	domain "github.com/superchalupa/go-redfish/redfishresource"
)

var (
	OBMC_ThermalPlugin = domain.PluginType("obmc_thermal")
)

type thermalSensorRedfish struct {
	MemberId                  string
	Name                      string
	SensorNumber              int
	Status                    StdStatus
	ReadingCelsius            float64
	UpperThresholdNonCritical int
	UpperThresholdCritical    int
	UpperThresholdFatal       int
	MinReadingRangeTemp       int
	MaxReadingRangeTemp       int
	PhysicalContext           string
}

type thermalList struct {
	sync.RWMutex
	sensors []*thermalSensor
}

type thermalSensor struct {
	redfish *thermalSensorRedfish
}

func NewThermalList() *thermalList {
	return &thermalList{
		sensors: []*thermalSensor{
			&thermalSensor{
				redfish: &thermalSensorRedfish{
					MemberId:                  "0",
					Name:                      "Fake Temp Sensor",
					SensorNumber:              42,
					ReadingCelsius:            25.0,
					UpperThresholdNonCritical: 35,
					UpperThresholdCritical:    40,
					UpperThresholdFatal:       50,
					MinReadingRangeTemp:       0,
					MaxReadingRangeTemp:       200,
					PhysicalContext:           "Fake Intake",
				},
			},
		},
	}
}

// satisfy the plugin interface so we can list ourselves as a plugin in our @meta
func (s *thermalList) PluginType() domain.PluginType { return OBMC_ThermalPlugin }

func (s *thermalList) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	s.RLock()
	defer s.RUnlock()

	// TODO: Pull the odata.id out of the agg we are passed to construct our sub-id

	res := []thermalSensorRedfish{}
	for _, t := range s.sensors {
		res = append(res, *t.redfish)
	}
	rrp.Value = res
}
