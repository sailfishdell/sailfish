package redfishserver

import (
	"context"
	"github.com/superchalupa/go-rfs/domain"
	"net/http"
    "fmt"
)

type basicAuthService struct {
	Service
}

// NewBasicAuthService returns a new instance of a basicAuth Service.
func NewBasicAuthService(s Service) Service {
	return &basicAuthService{Service: s}
}

func (s *basicAuthService) CheckAuthentication(ctx context.Context, r *http.Request) (resp *Response, privileges []string) {
    fmt.Printf("basicAuth CheckAuthentication\n")
	username, _, ok := r.BasicAuth()
	// TODO: check password (it's the unnamed second parameter, above, from r.BasicAuth())
	if ok {
        fmt.Printf("\tgot user: %s\n", username)
		account, err := domain.FindUser(ctx, s, username)
		if err == nil {
            privileges = domain.GetPrivileges(ctx, s, account)
            privileges = append(privileges, "authorization-complete")
            fmt.Printf("\tgot privs: %s\n", privileges)
            fmt.Printf("\tCongratulations, you have successfully authenticated.\n")
		} else {
            // 401 - Unauthorized: The authentication credentials included with this request are missing or invalid.
            // This handles "Invalid" case... need to handle "missing" case
            // later.  up in the stack, they can check for "authorization-complete"
            // privilege and raise 401 error (if authentication needed) or 403
            // (if they are authenticated but need more privs)
		    return &Response{StatusCode: http.StatusUnauthorized, Output: map[string]interface{}{"error": "Basic Auth failed: " + err.Error()}}, nil
        }
	}

    return nil, privileges
}

func (s *basicAuthService) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
    response, basicAuthPrivs :=  s.CheckAuthentication(ctx, r)
    if response != nil {
        return response, nil
    }

    if privileges != nil {
	    privileges = append(privileges, basicAuthPrivs...)
    }
	return s.Service.GetRedfishResource(ctx, r, privileges)
}

func (s *basicAuthService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
    response, basicAuthPrivs :=  s.CheckAuthentication(ctx, r)
    if response != nil {
        return response, nil
    }

    if privileges != nil {
	    privileges = append(privileges, basicAuthPrivs...)
    }
	return s.Service.RedfishResourceHandler(ctx, r, privileges)
}
