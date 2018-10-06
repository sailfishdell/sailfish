package stdcollections

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func AddAggregate(ctx context.Context, rootID eh.UUID, rootURI string, ch eh.CommandHandler) {
	// Create Computer System Collection
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: true,

			ResourceURI: rootURI + "/Systems",
			Type:        "#ComputerSystemCollection.ComputerSystemCollection",
			Context:     rootURI + "/$metadata#ComputerSystemCollection.ComputerSystemCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name": "Computer System Collection",
			}})

	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: rootID,
			Properties: map[string]interface{}{
				"Systems": map[string]interface{}{"@odata.id": rootURI + "/Systems"},
			},
		})

	// Create Computer System Collection
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: true,

			ResourceURI: rootURI + "/Chassis",
			Type:        "#ChassisCollection.ChassisCollection",
			Context:     rootURI + "/$metadata#ChassisCollection.ChassisCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name": "Chassis Collection",
			}})

	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: rootID,
			Properties: map[string]interface{}{
				"Chassis": map[string]interface{}{"@odata.id": rootURI + "/Chassis"},
			},
		})

	// Create Computer System Collection
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: true,

			ResourceURI: rootURI + "/Managers",
			Type:        "#ManagerCollection.ManagerCollection",
			Context:     rootURI + "/$metadata#ManagerCollection.ManagerCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":        "ManagerInstancesCollection",
				"Description": "Collection of BMCs",
			}})

	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: rootID,
			Properties: map[string]interface{}{
				"Managers": map[string]interface{}{"@odata.id": rootURI + "/Managers"},
			},
		})

	// Add Accounts collection
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: true,

			ResourceURI: rootURI + "/AccountService/Accounts",
			Type:        "#ManagerAccountCollection.ManagerAccountCollection",
			Context:     rootURI + "/$metadata#ManagerAccountCollection.ManagerAccountCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name": "Accounts Collection",
			}})

	// Add Roles collection
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: true,

			ResourceURI: rootURI + "/AccountService/Roles",
			Type:        "#RoleCollection.RoleCollection",
			Context:     rootURI + "/$metadata#RoleCollection.RoleCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name": "Roles Collection",
			}})

	// Create Computer System Collection
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: false,

			ResourceURI: rootURI + "/AccountService",
			Type:        "#AccountService.v1_0_2.AccountService",
			Context:     rootURI + "/$metadata#AccountService.AccountService",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{"ConfigureManager"}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":          "AccountService",
				"Name":        "Account Service",
				"Description": "Account Service",
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"ServiceEnabled":                  true,
				"AuthFailureLoggingThreshold":     3,
				"MinPasswordLength":               8,
				"AccountLockoutThreshold":         5,
				"AccountLockoutDuration":          30,
				"AccountLockoutCounterResetAfter": 30,
				"Accounts":                        map[string]string{"@odata.id": rootURI + "/AccountService/Accounts"},
				"Roles":                           map[string]string{"@odata.id": rootURI + "/AccountService/Roles"},
			}})

	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: rootID,
			Properties: map[string]interface{}{
				"AccountService": map[string]interface{}{"@odata.id": rootURI + "/AccountService"},
			},
		})

	// add standard DMTF roles: Admin
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: false,

			ResourceURI: rootURI + "/AccountService/Roles/Admin",
			Type:        "#Role.v1_0_2.Role",
			Context:     rootURI + "/$metadata#Role.Role",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":         "User Role",
				"Id":           "Admin",
				"Description":  "Admin User Role",
				"IsPredefined": true,
				"AssignedPrivileges": []string{
					"Login",
					"ConfigureManager",
					"ConfigureUsers",
					"ConfigureSelf",
					"ConfigureComponents",
				},
			}})

	// add standard DMTF roles: Operator
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: false,

			ResourceURI: rootURI + "/AccountService/Roles/Operator",
			Type:        "#Role.v1_0_2.Role",
			Context:     rootURI + "/$metadata#Role.Role",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":         "User Role",
				"Id":           "Operator",
				"Description":  "Operator User Role",
				"IsPredefined": true,
				"AssignedPrivileges": []string{
					"Login",
					"ConfigureSelf",
					"ConfigureComponents",
				},
			}})

	// add standard DMTF roles: Read-only
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: false,

			ResourceURI: rootURI + "/AccountService/Roles/ReadOnlyUser",
			Type:        "#Role.v1_0_2.Role",
			Context:     rootURI + "/$metadata#Role.Role",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":         "User Role",
				"Id":           "ReadOnlyUser",
				"Description":  "ReadOnlyUser User Role",
				"IsPredefined": true,
				"AssignedPrivileges": []string{
					"Login",
				},
			}})

}
