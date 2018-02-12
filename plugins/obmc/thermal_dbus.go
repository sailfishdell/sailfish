package obmc

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/godbus/dbus"
	mapper "github.com/superchalupa/go-redfish/plugins/dbus"
)

type dbusThermalList struct {
	*thermalList

	dbusSensors map[string]map[string]*thermalSensorRedfish
}

func NewDbusThermalList(ctx context.Context) *dbusThermalList {
	ret := &dbusThermalList{
		thermalList: NewThermalList(),
	}

	go ret.UpdateSensorList(ctx)

	return ret
}

func (d *dbusThermalList) UpdateSensorList(ctx context.Context) {
	var conn *dbus.Conn
	var err error
	for {
		// do{}while(0) equivalent so that we can skip the rest on errors
		// (break), but still hit the outside loop end to check for context
		// cancellation and do our sleep.
		for ok := true; ok; ok = false {
			if conn == nil {
				conn, err = dbus.SystemBus()
				if err != nil {
					fmt.Printf("Could not connect to system bus: %s\n", err)
					break
				}
			}
			m := mapper.New(conn)
			ret, err := m.GetSubTree(ctx, "/xyz/openbmc_project/sensors/temperature", 0, "xyz.openbmc_project.Sensor.Value")
			if err != nil {
				fmt.Printf("Mapper call failed: %s\n", err.Error())
				break
			}
			if len(ret) == 0 {
				fmt.Printf("empty array?")
				break
			}
			arr_0 := ret[0]
			dict, ok := arr_0.(map[string]map[string][]string)
			if !ok {
				fmt.Printf("type assert failed: %T\n", arr_0)
				break
			}

			// map[PATH]map[BUS][]interface
			newList := map[string]map[string]*thermalSensorRedfish{}
			for path, m1 := range dict {
				for bus, _ := range m1 {
					fmt.Printf("getting thermal for bus(%s)  path(%s)\n", bus, path)
					paths, ok := newList[bus]
					if !ok {
						paths = map[string]*thermalSensorRedfish{}
					}
					paths[path] = getThermal(ctx, conn, bus, path)
					newList[bus] = paths
				}
			}

			fmt.Printf("New thermals: %s\n", newList)

			d.Lock()
			d.dbusSensors = newList
			d.sensors = []*thermalSensor{}

			// dbusSensors map[string]map[string]*thermalSensorRedfish
			for _, d1 := range d.dbusSensors {
				for _, tsr := range d1 {
					d.sensors = append(d.sensors, &thermalSensor{redfish: tsr})
				}
			}

			d.Unlock()
		}

		// sleep for 10 seconds, or until context is cancelled
		select {
		case <-ctx.Done():
			fmt.Printf("Cancelling UpdateSensorList due to context cancellation.\n")
			break
		case <-time.After(10 * time.Second):
			continue
		}
	}
}

const (
	SensorValue = "xyz.openbmc_project.Sensor.Value"
)

func getThermal(ctx context.Context, conn *dbus.Conn, bus string, path string) *thermalSensorRedfish {
	busObject := conn.Object(bus, dbus.ObjectPath(path))

	scale, err := busObject.GetProperty(SensorValue + ".Scale")
	if err != nil {
		fmt.Printf("Error getting .Scale property for bus(%s) path(%s): %s\n", bus, path, err.Error())
		return nil
	}
	unit, err := busObject.GetProperty(SensorValue + ".Unit")
	if err != nil {
		fmt.Printf("Error getting .Unit property for bus(%s) path(%s): %s\n", bus, path, err.Error())
		return nil
	}
	value, err := busObject.GetProperty(SensorValue + ".Value")
	if err != nil {
		fmt.Printf("Error getting .Value property for bus(%s) path(%s): %s\n", bus, path, err.Error())
		return nil
	}

	if unit.Value() != "xyz.openbmc_project.Sensor.Value.Unit.DegreesC" {
		fmt.Printf("Don't know how to handle units: %s\n", unit)
		return nil
	}

	v, ok := value.Value().(int64)
	if !ok {
		fmt.Printf("Type assert of value to int failed: %T\n", value.Value())
		return nil
	}

	s, ok := scale.Value().(int64)
	if !ok {
		fmt.Printf("Type assert of scale to int failed.\n")
		return nil
	}
	var readingCelcius float64 = float64(v) * math.Pow(10, float64(s))

	return &thermalSensorRedfish{
		Name:           "dbus name",
		ReadingCelsius: readingCelcius,
	}
}
