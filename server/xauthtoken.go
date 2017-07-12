package redfishserver

import (
	"context"
	"fmt"
	"net/http"
)

var _ = fmt.Println

type xAuthTokenService struct {
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

// NewXAuthTokenService returns a new instance of a xAuthToken Service.
func NewXAuthTokenService(s Service) Service {
	return &xAuthTokenService{Service: s}
}

func (s *xAuthTokenService) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	return s.Service.GetRedfishResource(ctx, r, privileges)
}

func (s *xAuthTokenService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	return s.Service.RedfishResourceHandler(ctx, r, privileges)
}
