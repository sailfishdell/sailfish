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
	if ok {
		// TODO: Actually look up privileges
		// hardcode some privileges for now
		if username == "Administrator" && password == "password" {
			privileges := []string{
				"Unauthenticated", "basicauth",
				// per redfish spec
				"Login", "ConfigureManager", "ConfigureUsers", "ConfigureComponents", "ConfigureSelf",
			}
			a.OnUserDetails(username, privileges).ServeHTTP(rw, req)
			return
		}
		if username == "Operator" && password == "password" {
			privileges := []string{
				"Unauthenticated", "basicauth",
				"Login", "ConfigureComponents", "ConfigureSelf",
			}
			a.OnUserDetails(username, privileges).ServeHTTP(rw, req)
			return
		}
		if username == "ReadOnly" && password == "password" {
			privileges := []string{
				"Unauthenticated", "basicauth",
				"Login", "ConfigureSelf",
			}
			a.OnUserDetails(username, privileges).ServeHTTP(rw, req)
			return
		}
	}
	a.WithoutUserDetails.ServeHTTP(rw, req)
}

func NewService() (aud *AddUserDetails) {
	aud = &AddUserDetails{}
	return
}
