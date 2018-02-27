package chassis

import (
	"context"

	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

var (
	OBMC_ChassisPlugin = domain.PluginType("obmc_chassis")
)

// OCP Profile Redfish chassis object

type bmcInt interface {
	GetOdataID() string
}

type chassisService struct {
	*plugins.Service
	id  eh.UUID
	bmc bmcInt

	URIName      string `property:"uri_name"`
	Name         string `property:"name"`
	ChassisType  string `property:"chassis_type"`
	Manufacturer string `property:"manufacturer"`
	Model        string `property:"model"`
	SerialNumber string `property:"serial_number"`
	SKU          string `property:"sku"`
	AssetTag     string `property:"asset_tag"`
	PartNumber   string `property:"part_number"`
}

type ChassisOption func(*chassisService) error

func NewChassisService(ctx context.Context, options ...ChassisOption) (*chassisService, error) {
	c := &chassisService{
		Service:        plugins.NewService(plugins.PluginType(OBMC_ChassisPlugin)),
	}

	c.ApplyOption(options...)
	return c, nil
}

func (c *chassisService) ApplyOption(options ...ChassisOption) error {
	for _, o := range options {
		err := o(c)
		if err != nil {
			return err
		}
	}
	return nil
}

func ManagedBy(b bmcInt) ChassisOption {
	return func(p *chassisService) error {
		p.bmc = b
		return nil
	}
}

func (s *chassisService) GetOdataID() string { return "/redfish/v1/Chassis/" + s.URIName }

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

	plugins.RefreshProperty(ctx, *s, rrp, method, meta)
}

func (s *chassisService) AddResource(ctx context.Context, ch eh.CommandHandler) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  false,
			ResourceURI: s.GetOdataID(),
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
				"Name@meta":         map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "name"}},
				"Id":                s.URIName,
				"ChassisType@meta":  map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "chassis_type"}},
				"Manufacturer@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "manufacturer"}},
				"Model@meta":        map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "model"}},
				"SerialNumber@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "serial_number"}},
				"SKU@meta":          map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "sku"}},
				"PartNumber@meta":   map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "part_number"}},
				"AssetTag@meta":     map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "asset_tag"}},
				"IndicatorLED":      "Lit",
				"PowerState":        "On",
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},

				//"Thermal": map[string]interface{}{"@odata.id": "/redfish/v1/Chassis/A33/Thermal"},
				//"Power":   map[string]interface{}{"@odata.id": "/redfish/v1/Chassis/A33/Power"},
				"Links": map[string]interface{}{
					"ComputerSystems":   []map[string]interface{}{},
					"ManagedBy":         []map[string]interface{}{{"@odata.id": s.bmc.GetOdataID()}},
					"ManagersInChassis": []map[string]interface{}{{"@odata.id": s.bmc.GetOdataID()}},
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
