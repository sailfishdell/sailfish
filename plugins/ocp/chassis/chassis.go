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

type service struct {
	*plugins.Service
	bmc bmcInt
}

func New(options ...interface{}) (*service, error) {
	c := &service{
		Service: plugins.NewService(plugins.PluginType(OBMC_ChassisPlugin)),
	}

	c.ApplyOption(plugins.UUID()) // set uuid property... (GetUUID())
	c.ApplyOption(options...)
	c.ApplyOption(plugins.PropertyOnce("uri", "/redfish/v1/Chassis/"+c.GetProperty("unique_name").(string)))
	return c, nil
}

func WithUniqueName(uri string) plugins.Option {
	return plugins.PropertyOnce("unique_name", uri)
}

func ManagedBy(b bmcInt) Option {
	return func(p *service) error {
		p.bmc = b
		return nil
	}
}

type odataObj interface {
	GetOdataID() string
}

// no locking because it's an Option, loc
func manageOdataIDList(name string, obj odataObj) Option {
	return func(s *service) error {
		serversList, ok := s.GetPropertyOkUnlocked(name)
		if !ok {
			serversList = []map[string]string{}
		}
		sl, ok := serversList.([]map[string]string)
		if !ok {
			sl = []map[string]string{}
		}
		sl = append(sl, map[string]string{"@odata.id": obj.GetOdataID()})

		s.UpdatePropertyUnlocked(name, sl)
		return nil
	}
}

func AddManagedBy(obj odataObj) Option {
	return manageOdataIDList("managed_by", obj)
}

func (s *service) AddManagedBy(obj odataObj) {
	s.ApplyOption(AddManagedBy(obj))
}

func AddManagerInChassis(obj odataObj) Option {
	return manageOdataIDList("managers_in_chassis", obj)
}

func (s *service) AddManagerInChassis(obj odataObj) {
	s.ApplyOption(AddManagerInChassis(obj))
}

func AddComputerSystem(obj odataObj) Option {
	return manageOdataIDList("computer_systems", obj)
}

func (s *service) AddComputerSystem(obj odataObj) {
	s.ApplyOption(AddComputerSystem(obj))
}

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          s.GetUUID(),
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
				"Name@meta":         s.Meta(plugins.PropGET("name")),
				"Id":                s.GetProperty("unique_name"),
				"ChassisType@meta":  s.Meta(plugins.PropGET("chassis_type")),
				"Manufacturer@meta": s.Meta(plugins.PropGET("manufacturer")),
				"Model@meta":        s.Meta(plugins.PropGET("model")),
				"SerialNumber@meta": s.Meta(plugins.PropGET("serial_number")),
				"SKU@meta":          s.Meta(plugins.PropGET("sku")),
				"PartNumber@meta":   s.Meta(plugins.PropGET("part_number")),
				"AssetTag@meta":     s.Meta(plugins.PropGET("asset_tag")),
				"IndicatorLED":      "Lit",
				"PowerState":        "On",
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},

				"Links": map[string]interface{}{
					"ComputerSystems@meta":   s.Meta(plugins.PropGET("computer_systems")),
					"ManagedBy@meta":         s.Meta(plugins.PropGET("managed_by")),
					"ManagersInChassis@meta": s.Meta(plugins.PropGET("managers_in_chassis")),
				},
			}})
}
