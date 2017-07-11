package redfishserver

import (
	"context"
	"errors"
	"fmt"
	eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"
	"net/http"

	"github.com/superchalupa/go-rfs/domain"
)

type basicAuthService struct {
	Service
	baseURI     string
	verURI      string
	treeID      eh.UUID
	cmdbus      *commandbus.CommandBus
	redfishRepo *repo.Repo
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
func NewBasicAuthService(s Service, commandbus *commandbus.CommandBus, repo *repo.Repo, id eh.UUID, baseURI string) Service {
	return &basicAuthService{Service: s, cmdbus: commandbus, redfishRepo: repo, treeID: id, baseURI: baseURI, verURI: "v1"}
}

func (s *basicAuthService) findUser(ctx context.Context, user string) (account *domain.RedfishResource, err error) {
	// start looking up user in auth service
	tree, err := domain.GetTree(ctx, s.redfishRepo, s.treeID)
	if err != nil {
		return nil, errors.New("Malformed tree")
	}

	// get the root service reference
	rootService, err := tree.GetRedfishResourceFromTree(ctx, s.redfishRepo, s.baseURI+"/v1/")
	if err != nil {
		return nil, errors.New("Malformed tree root resource")
	}

	// Pull up the Accounts Collection
	accounts, err := tree.WalkRedfishResourceTree(ctx, s.redfishRepo, rootService, "AccountService", "@odata.id", "Accounts", "@odata.id")
	if err != nil {
		return nil, errors.New("Malformed Account Service")
	}

	// Walk through all of the "Members" of the collection, which are links to individual accounts
	members, ok := accounts.Properties["Members"]
	if !ok {
		return nil, errors.New("Malformed Account Collection")
	}

	// avoid panics by separating out type assertion
	memberList, ok := members.([]map[string]interface{})
	if !ok {
		return nil, errors.New("Malformed Account Collection")
	}

	for _, m := range memberList {
		a, _ := tree.GetRedfishResourceFromTree(ctx, s.redfishRepo, m["@odata.id"].(string))
		if a == nil {
			continue
		}
		if a.Properties == nil {
			continue
		}
		memberUser, ok := a.Properties["UserName"]
		if !ok {
			continue
		}
		if memberUser != user {
			continue
		}
		return a, nil
	}
	return nil, errors.New("User not found")
}

func (s *basicAuthService) getPrivileges(ctx context.Context, account *domain.RedfishResource) (privileges []string) {
	// start looking up user in auth service
	tree, err := domain.GetTree(ctx, s.redfishRepo, s.treeID)
	if err != nil {
		return
	}

	role, _ := tree.WalkRedfishResourceTree(ctx, s.redfishRepo, account, "Links", "Role", "@odata.id")
	privs, ok := role.Properties["AssignedPrivileges"]
	if !ok {
		return
	}

	for _, p := range privs.([]string) {
		// If the user has "ConfigureSelf", then append the special privilege that lets them configure their specific attributes
		if p == "ConfigureSelf" {
			// Add ConfigureSelf_%{USERNAME} property
			privileges = append(privileges, "ConfigureSelf_"+account.Properties["UserName"].(string))
		} else {
			// otherwise just pass through the actual priv
			privileges = append(privileges, p)
		}
	}

	var _ = fmt.Printf
	//fmt.Printf("Assigned the following Privileges: %s\n", privileges)
	return
}

func (s *basicAuthService) GetRedfishResource(ctx context.Context, headers map[string]string, url string, args map[string]string, privileges []string) (ret interface{}, err error) {
	username, ok := headers["BASIC_user"]
	// TODO: check password
	if ok {
		account, _ := s.findUser(ctx, username)
		privileges = append(privileges, s.getPrivileges(ctx, account)...)
	}

	/*
		pass, ok := headers["BASIC_pass"]
		if !ok {
			goto out
		}
		var _ = pass
	*/

	return s.Service.GetRedfishResource(ctx, headers, url, args, privileges)
}

func (s *basicAuthService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (output interface{}, StatusCode int, responseHeaders map[string]string, err error) {
	username, _, ok := r.BasicAuth()
	// TODO: check password (it's the unnamed second parameter, above, from r.BasicAuth())
	if ok {
		account, _ := s.findUser(ctx, username)
		privileges = append(privileges, s.getPrivileges(ctx, account)...)
	}
	return s.Service.RedfishResourceHandler(ctx, r, privileges)
}
