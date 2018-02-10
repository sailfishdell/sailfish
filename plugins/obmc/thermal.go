package obmc

import (
	"context"

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
        ReadingCelsius            int
        UpperThresholdNonCritical int
        UpperThresholdCritical    int
        UpperThresholdFatal       int
        MinReadingRangeTemp       int
        MaxReadingRangeTemp       int
        PhysicalContext           string
}

type thermalList []thermalSensor
type thermalSensor struct {
    redfish thermalSensorRedfish
}

// satisfy the plugin interface so we can list ourselves as a plugin in our @meta
func (s thermalList) PluginType() domain.PluginType { return OBMC_ThermalPlugin }

func (s thermalList) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
    res := []thermalSensorRedfish{}
    for _, t := range s {
        res = append(res, t.redfish)
    }
	rrp.Value = res
}
