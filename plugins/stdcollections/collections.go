package stdcollections

import (
    "context"

	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func NewService(ctx context.Context, rootID eh.UUID, ch eh.CommandHandler) {
		// Create Computer System Collection
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				Collection: true,

				ResourceURI: "/redfish/v1/Systems",
				Type:        "#ComputerSystemCollection.ComputerSystemCollection",
				Context:     "/redfish/v1/$metadata#ComputerSystemCollection.ComputerSystemCollection",
				Privileges: map[string]interface{}{
					"GET":    []string{"ConfigureManager"},
					"POST":   []string{"ConfigureManager"},
					"PUT":    []string{"ConfigureManager"},
					"PATCH":  []string{"ConfigureManager"},
					"DELETE": []string{},
				},
				Properties: map[string]interface{}{
                        "Name": "Computer System Collection",
				}})

		ch.HandleCommand(ctx,
			&domain.UpdateRedfishResourceProperties{
				ID: rootID,
				Properties: map[string]interface{}{
					"Systems": map[string]interface{}{"@odata.id": "/redfish/v1/Systems"},
				},
			})

		// Create Computer System Collection
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				Collection: true,

				ResourceURI: "/redfish/v1/Chassis",
				Type:        "#ChassisCollection.ChassisCollection",
				Context:     "/redfish/v1/$metadata#ChassisCollection.ChassisCollection",
				Privileges: map[string]interface{}{
					"GET":    []string{"ConfigureManager"},
					"POST":   []string{"ConfigureManager"},
					"PUT":    []string{"ConfigureManager"},
					"PATCH":  []string{"ConfigureManager"},
					"DELETE": []string{},
				},
				Properties: map[string]interface{}{
                    "Name": "Chassis Collection",
				}})


		ch.HandleCommand(ctx,
			&domain.UpdateRedfishResourceProperties{
				ID: rootID,
				Properties: map[string]interface{}{
					"Chassis": map[string]interface{}{"@odata.id": "/redfish/v1/Chassis"},
				},
			})

		// Create Computer System Collection
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				Collection: true,

				ResourceURI: "/redfish/v1/Managers",
				Type: "#ManagerCollection.ManagerCollection",
				Context:     "/redfish/v1/$metadata#ManagerCollection.ManagerCollection",
				Privileges: map[string]interface{}{
					"GET":    []string{"ConfigureManager"},
					"POST":   []string{"ConfigureManager"},
					"PUT":    []string{"ConfigureManager"},
					"PATCH":  []string{"ConfigureManager"},
					"DELETE": []string{},
				},
				Properties: map[string]interface{}{
                        "Name": "Manager Collection",
				}})

		ch.HandleCommand(ctx,
			&domain.UpdateRedfishResourceProperties{
				ID: rootID,
				Properties: map[string]interface{}{
					"Managers": map[string]interface{}{"@odata.id": "/redfish/v1/Managers"},
				},
			})

		// Create Computer System Collection
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				Collection: true,

				ResourceURI: "/redfish/v1/AccountService",
				Type: "#AccountService.v1_0_2.AccountService",
				Context:     "/redfish/v1/$metadata#AccountService.AccountService",
				Privileges: map[string]interface{}{
					"GET":    []string{"ConfigureManager"},
					"POST":   []string{"ConfigureManager"},
					"PUT":    []string{"ConfigureManager"},
					"PATCH":  []string{"ConfigureManager"},
					"DELETE": []string{},
				},
				Properties: map[string]interface{}{
                    "Id": "AccountService",
                    "Name": "Account Service",
                    "Description": "Account Service",
                    "Status": map[string]interface{}{
                        "State": "Enabled",
                        "Health": "OK",
                    },
                    "ServiceEnabled": true,
                    "AuthFailureLoggingThreshold": 3,
                    "MinPasswordLength": 8,
                    "AccountLockoutThreshold": 5,
                    "AccountLockoutDuration": 30,
                    "AccountLockoutCounterResetAfter": 30,
/*
                    "Accounts": {
                        "@odata.id": "/redfish/v1/AccountService/Accounts",
                    },
                    "Roles": {
                        "@odata.id": "/redfish/v1/AccountService/Roles",
                    },
*/
				}})

		ch.HandleCommand(ctx,
			&domain.UpdateRedfishResourceProperties{
				ID: rootID,
				Properties: map[string]interface{}{
					"AccountService": map[string]interface{}{"@odata.id": "/redfish/v1/AccountService"},
				},
			})
}
