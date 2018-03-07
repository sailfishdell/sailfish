// Build tags: only build this for the openbmc build. Be sure to note the required blank line after.
// +build openbmc

package obmc

import (
	"time"
)

const (
	DbusTimeout time.Duration = 1

	DbusUnitTemperatureValue = "xyz.openbmc_project.Sensor.Value.Unit.DegreesC"
	DbusUnitRPMValue         = "xyz.openbmc_project.Sensor.Value.Unit.RPMS"

	DbusInterfaceSensorThreshold = "xyz.openbmc_project.Sensor.Threshold"
	DbusInterfaceSensorValue     = "xyz.openbmc_project.Sensor.Value"

	DbusPathTemp = "/xyz/openbmc_project/sensors/temperature"
	DbusPathFan  = "/xyz/openbmc_project/sensors/fan_tach"
)
