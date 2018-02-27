// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build simulation

package obmc

import (
	"context"
	"fmt"
	"time"
)

const NUM_SIMULATION_SENSORS = 2

type simulationThermalList struct {
	*thermalList

	simulationSensors map[string]map[string]*thermalSensorRedfish
}

func NewThermalListImpl(ctx context.Context) *simulationThermalList {
	ret := &simulationThermalList{
		thermalList: NewThermalList(),
	}

	go ret.UpdateSensorList(ctx)

	return ret
}

func (d *simulationThermalList) UpdateSensorList(ctx context.Context) {
gofunc:
	for {
		d.Lock()
		d.sensors = []*thermalSensor{}

		// TODO: wiggle the sensors a little to simulate random temp changes
		// TODO: send redfish events when temps change
		for sensor := 0; sensor < NUM_SIMULATION_SENSORS; sensor++ {
			tsr := &thermalSensorRedfish{
				Name:                      fmt.Sprintf("Fake Temp Sensor %d", sensor),
				SensorNumber:              42,
				ReadingCelsius:            25.0,
				UpperThresholdNonCritical: 35,
				UpperThresholdCritical:    40,
				UpperThresholdFatal:       50,
				MinReadingRangeTemp:       0,
				MaxReadingRangeTemp:       200,
				PhysicalContext:           fmt.Sprintf("fake context %d", sensor),
			}

			d.sensors = append(d.sensors, &thermalSensor{redfish: tsr})
		}
		d.Unlock()

		// sleep for 10 seconds, or until context is cancelled
		select {
		case <-ctx.Done():
			fmt.Printf("Cancelling UpdateSensorList due to context cancellation.\n")
			break gofunc
		case <-time.After(10 * time.Second):
			continue
		}
	}
}
