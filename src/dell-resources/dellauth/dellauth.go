package dellauth

import (
	"net/http"
	"strconv"
)

// #define RF_PRIVILEGE_CONFIG_MANAGER                                     0x09 //Login+ConfigureManager
// #define RF_PRIVILEGE_CONFIG_SELF                                        0x11 //Login+ConfigureSelf
// #define RF_PRIVILEGE_CONFIG_MANAGER_USER                        0x0B  //Login+ConfigureManager+ConfigerUser

func MakeHandlerFunc(withUser func(string, []string) http.Handler, chain http.Handler) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		// User_priv
		// Authnz_user

		username := req.Header.Get("Authnz_user")
		privs := req.Header.Get("User_priv")

		privsInt, _ := strconv.Atoi(privs)
		privileges := PrivilegeBitsToStrings(privsInt)

		if username != "" {
			privileges = append(privileges,
				"Unauthenticated", "dellauth", "ConfigureSelf_"+username,
			)
		}

		if len(privileges) > 0 && username != "" {
			withUser(username, privileges).ServeHTTP(rw, req)
		} else {
			chain.ServeHTTP(rw, req)
		}
	}
}

/*
   Convert iDRAC privilege bites to their Redfish/OEM string versions
*/
func mEval(val, bits int) bool { return val&bits == bits }
func PrivilegeBitsToStrings(required int) []string {
	var Privileges []string

	if required == 0 {
		Privileges = append(Privileges, "Unauthenticated")
	} else {
		if mEval(required, 1) {
			Privileges = append(Privileges, "Login")
		}
		if mEval(required, 2) {
			Privileges = append(Privileges, "ConfigureManager")
		}
		if mEval(required, 4) {
			Privileges = append(Privileges, "ConfigureUsers")
		}
		if mEval(required, 8) {
			Privileges = append(Privileges, "ClearLogs")
		}
		if mEval(required, 16) {
			Privileges = append(Privileges, "ConfigureComponents")
		}
		if mEval(required, 32) {
			Privileges = append(Privileges, "AccessVirtualConsole")
		}
		if mEval(required, 64) {
			Privileges = append(Privileges, "AccessVirtualMedia")
		}
		if mEval(required, 128) {
			Privileges = append(Privileges, "TestAlerts")
		}
		if mEval(required, 256) {
			Privileges = append(Privileges, "ExecuteDebugCommands")
		}
	}

	return Privileges
}
