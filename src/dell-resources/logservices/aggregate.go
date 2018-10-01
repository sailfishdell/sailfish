package logservices

import (
	"context"

	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, v *view.View, baseUri string, ch eh.CommandHandler) (ret eh.UUID) {
	ret = eh.NewUUID()

	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          ret,
			Collection:  true,
			ResourceURI: baseUri + "/LogServices",
			Type:        "#LogServiceCollection.LogServiceCollection",
			Context:     "/redfish/v1/$metadata#LogServiceCollection.LogServiceCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"ConfigureManager"},
				"POST":   []string{},
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{},
			},
			Properties: map[string]interface{}{
				"Name":        "Log Service Collection",
				"Description": "Collection of Log Services for this Manager",
			}})

	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  false,
			ResourceURI: baseUri + "/LogServices/Lclog",
			Type:        "#LogService.v1_0_2.LogService",
			Context:     "/redfish/v1/$metadata#LogService.LogService",
			Privileges: map[string]interface{}{
				"GET":    []string{"ConfigureManager"},
				"POST":   []string{},
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{},
			},
			Properties: map[string]interface{}{
				"Name":               "LifeCycle Controller Log Service",
				"Description":        "LifeCycle Controller Log Service",
				"OverWritePolicy":    "WrapsWhenFull",
				"MaxNumberOfRecords": 500000,
				"ServiceEnabled":     true,
				"Entries": map[string]interface{}{
					"@odata.id": "/redfish/v1/Managers/CMC.Integrated.1/Logs/Lclog",
				},
				"DateTimeLocalOffset": "+00:00",
				"Id": "LC",
			}})

        ch.HandleCommand(
                ctx,
                &domain.CreateRedfishResource{
                        ID:          eh.NewUUID(),
                        Collection:  false,
                        ResourceURI: baseUri + "/LogServices/FaultList",
                        Type:        "#LogService.v1_0_2.LogService",
                        Context:     "/redfish/v1/$metadata#LogService.LogService",
                        Privileges: map[string]interface{}{
                                "GET":    []string{"ConfigureManager"},
                                "POST":   []string{},
                                "PUT":    []string{},
                                "PATCH":  []string{},
                                "DELETE": []string{},
                        },
                        Properties: map[string]interface{}{
                                "Name":               "FaultListEntries",
                                "Description":        "Collection of FaultList Entries",
                                "OverWritePolicy":    "WrapsWhenFull",
                                "MaxNumberOfRecords": 500000,
                                "ServiceEnabled":     true,
                                "Entries": map[string]interface{}{
                                        "@odata.id": "/redfish/v1/Managers/CMC.Integrated.1/Logs/FaultList",
                                },
                                "DateTimeLocalOffset": "+00:00",
				"DateTime": "TODO", //TODO
                                "Id": "FaultList",
                        }})
	return
}
