package dell_ec

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func RegisterCertAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("certificateservices",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#DellCertificateService.v1_0_0.DellCertificateService",
					Context:     "/redfish/v1/$metadata#DellCertificateService.DellCertificateService",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Id":                        "CertificateService",
						"Name":                      "Certificate Service",
						"Description":               "Represents the properties of Certificate Service",
						"CertificateSigningRequest": nil,
						"Actions": map[string]interface{}{
							"#DellCertificateService.GenerateCSR": map[string]interface{}{
								"target": vw.GetActionURI("certificates.generatecsr"),
								"Type@Redfish.AllowableValues": []string{
									"FactoryIdentity",
								},
							},
						},
						"CertificateInventory": map[string]interface{}{
							"@odata.id": vw.GetURI() + "/CertificateInventory",
						},
					},
				}}, nil
		})

	s.RegisterAggregateFunction("certificatecollection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#DellCertificateInventoryCollection.DellCertificateInventoryCollection",
					Context:     "/redfish/v1/$metadata#DellCertificateInventoryCollection.DellCertificateInventoryCollection",
					Plugin:      "GenericActionHandler",
					Privileges: map[string]interface{}{
						"GET":  []string{"Login"},
						"POST": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Name":                     "Certificate Inventory Collection",
						"Description":              "Collection of Certificate Inventory",
						"Members":                  []interface{}{},
						"Members@odata.count":      0,
					},
				}}, nil
		})

	// todo: make this more generic?
	s.RegisterAggregateFunction("certificate",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#DellCertificateInventory.v1_0_0.DellCertificateInventory",
					Context:     "/redfish/v1/$metadata#DellCertificateInventory.DellCertificateInventory",
					Plugin:      "GenericActionHandler",
					Privileges: map[string]interface{}{
						"GET":  []string{"Login"},
						"POST": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{

						"Certificate@meta":    map[string]interface{}{"GET": map[string]interface{}{"plugin": "certinfo"}},
						"Description":        "Certificate Inventory Instance",
						"DownloadFileFormat": "PEM",
						"Id":                 "FactoryIdentity.1",
						"Name":               "Factory Identity Certificate",
						"Type":               "FactoryIdentity",
						"Actions": map[string]interface{}{
							"#DellCertificateService.GetCertInfo": map[string]interface{}{
								"target": vw.GetActionURI("certificates.getcert"),
							},
						},
					},
				}}, nil
		})
}
