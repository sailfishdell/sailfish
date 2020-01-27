package stdcollections

import (
	"context"

	eh "github.com/looplab/eventhorizon"

	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func AddStandardRoles(ctx context.Context, rootID eh.UUID, rootURI string, ch eh.CommandHandler) {
	// Create Computer System Collection

	// add standard DMTF roles: Admin
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID: eh.NewUUID(),

			ResourceURI: rootURI + "/AccountService/Roles/Admin",
			Type:        "#Role.v1_0_2.Role",
			Context:     rootURI + "/$metadata#Role.Role",
			Privileges: map[string]interface{}{
				"GET": []string{"Login"},
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
			ID:          eh.NewUUID(),
			ResourceURI: rootURI + "/AccountService/Roles/Operator",
			Type:        "#Role.v1_0_2.Role",
			Context:     rootURI + "/$metadata#Role.Role",
			Privileges: map[string]interface{}{
				"GET": []string{"Login"},
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
			ID:          eh.NewUUID(),
			ResourceURI: rootURI + "/AccountService/Roles/ReadOnlyUser",
			Type:        "#Role.v1_0_2.Role",
			Context:     rootURI + "/$metadata#Role.Role",
			Privileges: map[string]interface{}{
				"GET": []string{"Login"},
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
