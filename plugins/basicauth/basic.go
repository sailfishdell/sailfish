package basicauth

import (
    "net/http"
    "context"
)


type AddUserDetails struct {
	OnUserDetails      func(userName string, privileges []string) http.Handler
	WithoutUserDetails http.Handler
}


func (a *AddUserDetails) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    username, password, ok := req.BasicAuth()
    if ok {
        privileges := []string{"Unauthenticated", "ConfigureManager", "basicauth"}
        if username == "root" && password == "password" {
		    a.OnUserDetails(username, privileges).ServeHTTP(rw, req)
            return
        }
    }
    a.WithoutUserDetails.ServeHTTP(rw, req)
}

func NewService(ctx context.Context) (aud *AddUserDetails) {
    aud = &AddUserDetails{}
    return
}
