package redfishserver

import (
	"context"
	"errors"
	"fmt"
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
func NewBasicAuthService(s Service, commandbus *commandbus.CommandBus, repo *repo.Repo, id eh.UUID, baseURI string) Service {
	return &basicAuthService{Service: s, cmdbus: commandbus, odataRepo: repo, treeID: id, baseURI: baseURI, verURI: "v1"}
}

func (s *basicAuthService) getTree(ctx context.Context) (t *domain.OdataTree, err error) {
	rawTree, err := s.odataRepo.Find(ctx, s.treeID)
	if err != nil {
		return nil, errors.New("could not find tree with ID: " + string(s.treeID) + " error is: " + err.Error())
	}

	t, ok := rawTree.(*domain.OdataTree)
	if !ok {
		fmt.Printf("somehow it wasnt a tree! %s\n", err.Error())
		return nil, errors.New("Data structure inconsistency, the tree object wasnt a tree!: " + string(s.treeID) + " error is: " + err.Error())
	}

	return
}

func (s *basicAuthService) GetOdataResource(ctx context.Context, headers map[string]string, url string, args map[string]string, privileges []string) (ret interface{}, err error) {
	// the only thing we do in this service is look up the username/password and verify, then look up role, then assign privileges based on role
	var user, pass string
	var ok bool
	var tree *domain.OdataTree
	var rootService *domain.OdataResource
	var accounts *domain.OdataResource
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
	tree, err = s.getTree(ctx)
	rootService, err = tree.GetOdataResourceFromTree(ctx, s.odataRepo, s.baseURI+"/v1/")

	// Pull up the Accounts Collection
	accounts, err = tree.WalkOdataResourceTree(ctx, s.odataRepo, rootService, "AccountService", "@odata.id", "Accounts", "@odata.id")
	if err != nil {
		goto out
	}

	// Walk through all of the "Members" of the collection, which are links to individual accounts
	members, ok = accounts.Properties["Members"]
	if !ok {
		goto out
	}

	// avoid panics by separating out
	memberList, ok = members.([]map[string]interface{})
	if !ok {
		goto out
	}

	for _, m := range memberList {
		a, _ := tree.GetOdataResourceFromTree(ctx, s.odataRepo, m["@odata.id"].(string))
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
		role, _ := tree.WalkOdataResourceTree(ctx, s.odataRepo, a, "Links", "Role", "@odata.id")
		privs, ok := role.Properties["AssignedPrivileges"]
		if !ok {
			continue
		}

		fmt.Printf("\tAssigned the following Privileges: %s\n", privs)
		for _, p := range privs.([]string) {
			privileges = append(privileges, p)
		}
	}

out:
	return s.Service.GetOdataResource(ctx, headers, url, args, privileges)
}
