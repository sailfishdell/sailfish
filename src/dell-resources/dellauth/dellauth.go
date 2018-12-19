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
		privileges := []string{}
		if privsInt&0x09 == 0x09 {
			privileges = append(privileges, "Login", "ConfigureManager")
		}
		if privsInt&0x11 == 0x11 {
			privileges = append(privileges, "Login", "ConfigureSelf")
		}
		if privsInt&0x0b == 0x0b {
			privileges = append(privileges, "Login", "ConfigureManager", "ConfigureUser")
		}

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
