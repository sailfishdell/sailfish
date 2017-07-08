package redfishserver

import (
	"context"
	"fmt"
    "net/http"
	eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"

	"github.com/superchalupa/go-rfs/domain"
)

type basicAuthService struct {
	Service
	baseURI   string
	verURI    string
	treeID    eh.UUID
	cmdbus    *commandbus.CommandBus
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

func (s *basicAuthService) GetRedfishResource(ctx context.Context, headers map[string]string, url string, args map[string]string, privileges []string) (ret interface{}, err error) {
	// the only thing we do in this service is look up the username/password and verify, then look up role, then assign privileges based on role
	var user, pass string
	var ok bool
	var tree *domain.RedfishTree
	var rootService *domain.RedfishResource
	var accounts *domain.RedfishResource
	var members interface{}
	var memberList []map[string]interface{}

	user, ok = headers["BASIC_user"]
	if !ok {
		goto out
	}
	pass, ok = headers["BASIC_pass"]
	if !ok {
		goto out
	}
	var _ = pass

	// start looking up user in auth service
	tree, err = domain.GetTree(ctx, s.redfishRepo, s.treeID)
    if err != nil {
        goto out
    }

	rootService, err = tree.GetRedfishResourceFromTree(ctx, s.redfishRepo, s.baseURI+"/v1/")
    if err != nil {
        goto out
    }

	// Pull up the Accounts Collection
	accounts, err = tree.WalkRedfishResourceTree(ctx, s.redfishRepo, rootService, "AccountService", "@odata.id", "Accounts", "@odata.id")
	if err != nil {
		goto out
	}

	// Walk through all of the "Members" of the collection, which are links to individual accounts
	members, ok = accounts.Properties["Members"]
	if !ok {
		goto out
	}

	// avoid panics by separating out type assertion
	memberList, ok = members.([]map[string]interface{})
	if !ok {
		goto out
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
		role, _ := tree.WalkRedfishResourceTree(ctx, s.redfishRepo, a, "Links", "Role", "@odata.id")
		privs, ok := role.Properties["AssignedPrivileges"]
		if !ok {
			continue
		}

		for _, p := range privs.([]string) {
            // If the user has "ConfigureSelf", then append the special privilege that lets them configure their specific attributes
            if p == "ConfigureSelf" {
                // Add ConfigureSelf_%{USERNAME} property
                // FIXME: check type assertion
                privileges = append(privileges, "ConfigureSelf_" + memberUser.(string))
            } else {
                // otherwise just pass through the actual priv
			    privileges = append(privileges, p)
            }
		}


		fmt.Printf("\tAssigned the following Privileges: %s\n", privileges)
	}

out:
	return s.Service.GetRedfishResource(ctx, headers, url, args, privileges)
}


func (s *basicAuthService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (ret interface{}, err error) {
	return s.Service.RedfishResourceHandler(ctx, r, privileges)
}
