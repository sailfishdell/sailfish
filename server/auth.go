package redfishserver

import (
	"context"
	"fmt"
	eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"
)

type basicAuthService struct {
	Service
	treeID    eh.UUID
	cmdbus    *commandbus.CommandBus
	odataRepo *repo.Repo
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
func NewBasicAuthService(s Service, commandbus *commandbus.CommandBus, repo *repo.Repo, id eh.UUID) Service {
	return &basicAuthService{Service: s, cmdbus: commandbus, odataRepo: repo, treeID: id}
}

func (s *basicAuthService) GetOdataResource(ctx context.Context, headers map[string]string, url string, args map[string]string, privileges []string) (ret interface{}, err error) {
	// the only thing we do in this service is look up the username/password and verify, then look up role, then assign privileges based on role
	var user, pass string
	var ok bool
	user, ok = headers["BASIC_user"]
	if !ok {
		goto out
	}
	pass, ok = headers["BASIC_pass"]
	if !ok {
		goto out
	}
	fmt.Printf("HEY! Got a really basic user: %s\n", user)
	fmt.Printf("HEY! Got a really basic pass: %s\n", pass)

out:
	return s.Service.GetOdataResource(ctx, headers, url, args, privileges)
}
