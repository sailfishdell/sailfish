package basicauth

import (
	"net/http"
)

type AddUserDetails struct {
	OnUserDetails      func(userName string, privileges []string) http.Handler
	WithoutUserDetails http.Handler
}

func (a *AddUserDetails) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
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
		a.OnUserDetails(username, privileges).ServeHTTP(rw, req)
	} else {
		a.WithoutUserDetails.ServeHTTP(rw, req)
	}
}

func NewService() (aud *AddUserDetails) {
	aud = &AddUserDetails{}
	return
}
