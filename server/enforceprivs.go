package redfishserver

import (
	"context"
	"fmt"
	eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"
	"strings"
    "net/http"
    "errors"

	"github.com/superchalupa/go-rfs/domain"
)

var _ = fmt.Println

type privilegeEnforcingService struct {
	Service
	baseURI     string
	verURI      string
	treeID      eh.UUID
	cmdbus      *commandbus.CommandBus
	redfishRepo *repo.Repo
}

// NewPrivilegeEnforcingService returns a new instance of a privilegeEnforcing Service.
func NewPrivilegeEnforcingService(s Service, baseURI string, commandbus *commandbus.CommandBus, repo *repo.Repo, id eh.UUID) Service {
	return &privilegeEnforcingService{Service: s, cmdbus: commandbus, redfishRepo: repo, treeID: id, baseURI: baseURI, verURI: "v1"}
}

func (s *privilegeEnforcingService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (resp *Response, err error) {
	noHashPath := strings.SplitN(r.URL.Path, "#", 2)[0]

	// we have the tree ID, fetch an updated copy of the actual tree
	// TODO: Locking? Should repo give us a copy? Need to test this.
	tree, err := domain.GetTree(ctx, s.redfishRepo, s.treeID)
	if err != nil {
		return &Response{StatusCode: http.StatusInternalServerError}, err
	}

	// now that we have the tree, look up the actual URI in that tree to find
	// the object UUID, then pull that from the repo
	requested, err := s.redfishRepo.Find(ctx, tree.Tree[noHashPath])
	if err != nil {
		return &Response{StatusCode: http.StatusNotFound}, nil
	}
	item, ok := requested.(*domain.RedfishResource)
	if !ok {
		return &Response{StatusCode: http.StatusInternalServerError}, errors.New("Expected a RedfishResource, but got something strange.")
	}

	// security privileges. Check to see if user has permissions on the object
	// FIXME/TODO: SIMPLE IMPLEMENTATION... this needs to handle AND/OR combinations.
	// Also need to consider purity. This could realistically be implemented as two additional Service wrappers:
	//  1) a pre-call check that does the gross privilege check
	//  2) a post-call check that filters the properties returned based on privs
	getPrivs, ok := item.PrivilegeMap[r.Method]
	if !ok {
		return &Response{StatusCode: http.StatusForbidden}, nil
	}

	getPrivsArr := getPrivs.([]string)

	//fmt.Printf("CHECK PRIVS\n\tUSER: %s\n\tRESOURCE: %s\n", privileges, getPrivsArr)

	for _, myPriv := range privileges {
		for _, itemPriv := range getPrivsArr {
			if myPriv == itemPriv {
				//fmt.Printf("Found matching privs, granting access. userPriv(%s) == itemPriv(%s)\n", myPriv, itemPriv)
				return s.Service.RedfishResourceHandler(ctx, r, privileges)
			}
		}
	}

    return &Response{StatusCode: http.StatusForbidden}, nil
}

func (s *privilegeEnforcingService) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (resp *Response, err error) {
	noHashPath := strings.SplitN(r.URL.Path, "#", 2)[0]

	// we have the tree ID, fetch an updated copy of the actual tree
	// TODO: Locking? Should repo give us a copy? Need to test this.
	tree, err := domain.GetTree(ctx, s.redfishRepo, s.treeID)
	if err != nil {
		return &Response{StatusCode: http.StatusInternalServerError}, err
	}

	// now that we have the tree, look up the actual URI in that tree to find
	// the object UUID, then pull that from the repo
	requested, err := s.redfishRepo.Find(ctx, tree.Tree[noHashPath])
	if err != nil {
		return &Response{StatusCode: http.StatusNotFound}, nil
	}
	item, ok := requested.(*domain.RedfishResource)
	if !ok {
		return &Response{StatusCode: http.StatusInternalServerError}, errors.New("Expected a RedfishResource, but got something strange.")
	}

	// security privileges. Check to see if user has permissions on the object
	// FIXME/TODO: SIMPLE IMPLEMENTATION... this needs to handle AND/OR combinations.
	// Also need to consider purity. This could realistically be implemented as two additional Service wrappers:
	//  1) a pre-call check that does the gross privilege check
	//  2) a post-call check that filters the properties returned based on privs
	getPrivs, ok := item.PrivilegeMap[r.Method]
	if !ok {
		return &Response{StatusCode: http.StatusForbidden}, nil
	}

	getPrivsArr := getPrivs.([]string)

	//fmt.Printf("CHECK PRIVS\n\tUSER: %s\n\tRESOURCE: %s\n", privileges, getPrivsArr)

	for _, myPriv := range privileges {
		for _, itemPriv := range getPrivsArr {
			if myPriv == itemPriv {
				//fmt.Printf("Found matching privs, granting access. userPriv(%s) == itemPriv(%s)\n", myPriv, itemPriv)
				return s.Service.GetRedfishResource(ctx, r, privileges)
			}
		}
	}

    return &Response{StatusCode: http.StatusForbidden}, nil
}
