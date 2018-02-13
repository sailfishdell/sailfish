package obmc

import (
	"context"
	"fmt"
	"sync"

	domain "github.com/superchalupa/go-redfish/redfishresource"
)

var (
	OBMC_ThermalPlugin = domain.PluginType("obmc_thermal")
)

type thermalSensorRedfish struct {
	OdataID                   string `json:"@odata.id"`
	MemberID                  string
	Name                      string
	SensorNumber              int
	Status                    StdStatus
	ReadingCelsius            float64
	UpperThresholdNonCritical float64
	UpperThresholdCritical    float64
	UpperThresholdFatal       float64
	MinReadingRangeTemp       float64
	MaxReadingRangeTemp       float64
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
	return &thermalList{}
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
	for idx, t := range s.sensors {
		res = append(res, *t.redfish)
		idstr := fmt.Sprintf("%d", idx)
		t.redfish.MemberID = fmt.Sprintf("%s", idstr)
		t.redfish.OdataID = agg.ResourceURI + "#/Temperatures/" + idstr
	}
	rrp.Value = res
}
