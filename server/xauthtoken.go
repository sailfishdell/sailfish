package redfishserver

import (
	"context"
	"fmt"
	eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"
	"net/http"
)

var _ = fmt.Println

type xAuthTokenService struct {
	Service
	baseURI     string
	verURI      string
	treeID      eh.UUID
	cmdbus      *commandbus.CommandBus
	redfishRepo *repo.Repo
	*basicAuthService
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
func NewXAuthTokenService(s Service, commandbus *commandbus.CommandBus, repo *repo.Repo, id eh.UUID, baseURI string) Service {
	return &xAuthTokenService{
		Service:               s,
		cmdbus:                commandbus,
		redfishRepo:           repo,
		treeID:                id,
		baseURI:               baseURI,
		verURI:                "v1",
		basicAuthService: NewBasicAuthService(s, commandbus, repo, id, baseURI).(*basicAuthService),
	}
}

func (s *xAuthTokenService) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	return s.Service.GetRedfishResource(ctx, r, privileges)
}

func (s *xAuthTokenService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	return s.Service.RedfishResourceHandler(ctx, r, privileges)
}
