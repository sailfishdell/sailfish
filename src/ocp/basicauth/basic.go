package basicauth

import (
	"net/http"
)

func MakeHandlerFunc(withUser func(string, []string) http.Handler, chain http.Handler) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		username, password, ok := req.BasicAuth()
		privileges := []string{}
		if ok {
			// TODO: Actually look up privileges
			// hardcode some privileges for now
			if username == "Administrator" && password == "password" {
				privileges = append(privileges,
					"Unauthenticated", "basicauth", "ConfigureSelf_"+username,
					"Login", "ConfigureManager", "ConfigureUsers", "ConfigureComponents",
				)
			}
			if username == "Operator" && password == "password" {
				privileges = append(privileges,
					"Unauthenticated", "basicauth", "ConfigureSelf_"+username,
					"Login", "ConfigureComponents",
				)
			}
			if username == "ReadOnly" && password == "password" {
				privileges = append(privileges,
					"Unauthenticated", "basicauth", "ConfigureSelf_"+username,
					"Login",
				)
			}
		}
		if len(privileges) > 0 && username != "" {
			withUser(username, privileges).ServeHTTP(rw, req)
		} else {
			chain.ServeHTTP(rw, req)
		}
	}
}
