package obmc

import (
	"context"
	"fmt"
	"sync"

	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	//	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
)

var (
	OBMC_ChassisPlugin = domain.PluginType("obmc_chassis")
)

// OCP Profile Redfish chassis object

type StdStatus struct {
	State  string
	Health string
}

type chassisService struct {
	sync.Mutex
	thermalSensors thermalList
}

func NewChassisService(ctx context.Context) (*chassisService, error) {
	return &chassisService{
		thermalSensors: thermalList{
			thermalSensor{
				redfish: thermalSensorRedfish{
					MemberId:                  "0",
					Name:                      "Inlet Temp",
					SensorNumber:              42,
					ReadingCelsius:            25,
					UpperThresholdNonCritical: 35,
					UpperThresholdCritical:    40,
					UpperThresholdFatal:       50,
					MinReadingRangeTemp:       0,
					MaxReadingRangeTemp:       200,
					PhysicalContext:           "Intake",
				},
			},
			thermalSensor{
				redfish: thermalSensorRedfish{
					MemberId:                  "1",
					Name:                      "Random other Temp",
					SensorNumber:              53,
					ReadingCelsius:            26,
					UpperThresholdNonCritical: 35,
					UpperThresholdCritical:    40,
					UpperThresholdFatal:       50,
					MinReadingRangeTemp:       0,
					MaxReadingRangeTemp:       200,
					PhysicalContext:           "Other",
				},
			},
		},
	}, nil
}

// wait in a listener for the root service to be created, then extend it
func InitChassisService(ctx context.Context, s *chassisService, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// Singleton for bmc plugin: we can pull data out of ourselves on GET/etc.
	domain.RegisterPlugin(func() domain.Plugin { return s })
	domain.RegisterPlugin(func() domain.Plugin { return s.thermalSensors })

	// step 2: Add openbmc chassis object after Chassis collection has been created
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURI("/redfish/v1/Chassis"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return
	}
	sp.RunOnce(func(event eh.Event) {
		s.AddOBMCChassisResource(ctx, ch)
	})
}

// satisfy the plugin interface so we can list ourselves as a plugin in our @meta
func (s *chassisService) PluginType() domain.PluginType { return OBMC_ChassisPlugin }

func (s *chassisService) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	s.Lock()
	defer s.Unlock()

	rrp.Value = "NOT IMPLEMENTED YET"
}

func (s *chassisService) AddOBMCChassisResource(ctx context.Context, ch eh.CommandHandler) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  false,
			ResourceURI: "/redfish/v1/Chassis/A33",
			Type:        "#Chassis.v1_2_0.Chassis",
			Context:     "/redfish/v1/$metadata#Chassis.Chassis",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":         "Catfish System Chassis",
				"Id":           "A33",
				"ChassisType":  "RackMount",
				"Manufacturer": "CatfishManufacturer",
				"Model":        "YellowCat1000",
				"SerialNumber": "2M220100SL",
				"SKU":          "",
				"PartNumber":   "",
				"AssetTag":     "CATFISHASSETTAG",
				"IndicatorLED": "Lit",
				"PowerState":   "On",
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},

				"Thermal": map[string]interface{}{"@odata.id": "/redfish/v1/Chassis/A33/Thermal"},
				"Power":   map[string]interface{}{"@odata.id": "/redfish/v1/Chassis/A33/Power"},
				"Links": map[string]interface{}{
					"ComputerSystems":   []map[string]interface{}{},
					"ManagedBy":         []map[string]interface{}{{"@odata.id": "/redfish/v1/Managers/bmc"}},
					"ManagersInChassis": []map[string]interface{}{{"@odata.id": "/redfish/v1/Managers/bmc"}},
				},
			}})

	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  false,
			ResourceURI: "/redfish/v1/Chassis/A33/Thermal",
			Type:        "#Thermal.v1_1_0.Thermal",
			Context:     "/redfish/v1/$metadata#Thermal.Thermal",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":                "Thermal",
				"Name":              "Thermal",
				"Temperatures@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_thermal"}},
				"Fans": []map[string]interface{}{
					map[string]interface{}{
						"@odata.id":       "/redfish/v1/Chassis/A33/Thermal#/Fans/0",
						"MemberId":        "0",
						"Name":            "BaseBoard System Fan 1",
						"PhysicalContext": "Backplane",
						"Status": map[string]interface{}{
							"State":  "Enabled",
							"Health": "OK",
						},
						"Reading":                   2100,
						"ReadingUnits":              "RPM",
						"UpperThresholdNonCritical": 42,
						"UpperThresholdCritical":    4200,
						"UpperThresholdFatal":       42,
						"LowerThresholdNonCritical": 42,
						"LowerThresholdCritical":    5,
						"LowerThresholdFatal":       42,
						"MinReadingRange":           0,
						"MaxReadingRange":           5000,
						"Redundancy":                []map[string]interface{}{{"@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Redundancy/0"}},
					},
					map[string]interface{}{
						"@odata.id":       "/redfish/v1/Chassis/A33/Thermal#/Fans/1",
						"MemberId":        "1",
						"Name":            "BaseBoard System Fan 2",
						"PhysicalContext": "Backplane",
						"Status": map[string]interface{}{
							"State":  "Enabled",
							"Health": "OK",
						},
						"Reading":                   2100,
						"ReadingUnits":              "RPM",
						"UpperThresholdNonCritical": 42,
						"UpperThresholdCritical":    4200,
						"UpperThresholdFatal":       42,
						"LowerThresholdNonCritical": 42,
						"LowerThresholdCritical":    5,
						"LowerThresholdFatal":       42,
						"MinReadingRange":           0,
						"MaxReadingRange":           5000,
						"Redundancy":                []map[string]interface{}{{"@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Redundancy/0"}},
					},
				},
				"Redundancy": []map[string]interface{}{
					{
						"@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Redundancy/0",
						"MemberId":  "0",
						"Name":      "BaseBoard System Fans",
						"RedundancySet": []map[string]interface{}{
							{"@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Fans/0"},
							{"@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Fans/1"},
						},
						"Mode": "N+m",
						"Status": map[string]interface{}{
							"State":  "Enabled",
							"Health": "OK",
						},
						"MinNumNeeded":    1,
						"MaxNumSupported": 2,
					},
				},
			}})

	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  false,
			ResourceURI: "/redfish/v1/Chassis/A33/Power",
			Type:        "#Power.v1_1_0.Power",
			Context:     "/redfish/v1/$metadata#Power.Power",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":   "Power",
				"Name": "Power",
				"PowerControl": []map[string]interface{}{
					map[string]interface{}{
						"@odata.id":          "/redfish/v1/Chassis/A33/Power#/PowerControl/0",
						"MemberId":           "0",
						"Name":               "System Power Control",
						"PowerConsumedWatts": 224,
						"PowerCapacityWatts": 600,
						"PowerLimit": map[string]interface{}{
							"LimitInWatts":   450,
							"LimitException": "LogEventOnly",
							"CorrectionInMs": 1000,
						},
						"Status": map[string]interface{}{
							"State":  "Enabled",
							"Health": "OK",
						},
					},
				},
			},
		})
}
