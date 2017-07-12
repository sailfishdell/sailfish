package redfishserver

import (
	"context"
	"net/http"
)

type basicAuthService struct {
	Service
}

// step 1: basic auth against pre-defined account collection/role collection
// step 2: Add session support
//      -- POST handler to create session, which checks username/password and returns token. token should code the session id
//      -- in every request, reset timeout
//      -- if timeout passes, delete session
//      -- DELETE handler so user can manually end session
// step 3: Add generic oauth support

// instantiate this service, tell it the URI of the account collection and role collection

// NewBasicAuthService returns a new instance of a basicAuth Service.
func NewBasicAuthService(s Service) Service {
	return &basicAuthService{Service: s}
}

func (s *basicAuthService) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	username, _, ok := r.BasicAuth()
	// TODO: check password (it's the unnamed second parameter, above, from r.BasicAuth())
	if ok {
		account, _ := s.FindUser(ctx, username)
		privileges = append(privileges, s.GetPrivileges(ctx, account)...)
	}

	return s.Service.GetRedfishResource(ctx, r, privileges)
}

func (s *basicAuthService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	username, _, ok := r.BasicAuth()
	// TODO: check password (it's the unnamed second parameter, above, from r.BasicAuth())
	if ok {
		account, _ := s.FindUser(ctx, username)
		privileges = append(privileges, s.GetPrivileges(ctx, account)...)
	}
	return s.Service.RedfishResourceHandler(ctx, r, privileges)
}
