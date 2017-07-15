package redfishserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/superchalupa/go-redfish/domain"
)

var _ = fmt.Println

type privilegeEnforcingService struct {
	Service
}

// NewPrivilegeEnforcingService returns a new instance of a privilegeEnforcing Service.
func NewPrivilegeEnforcingService(s Service) Service {
	return &privilegeEnforcingService{Service: s}
}


func (s *privilegeEnforcingService) IsAuthorized(ctx context.Context, r *http.Request, privileges []string) (resp *Response, authorized bool) {
	noHashPath := strings.SplitN(r.URL.Path, "#", 2)[0]

	// we have the tree ID, fetch an updated copy of the actual tree
	// TODO: Locking? Should repo give us a copy? Need to test this.
	tree, err := domain.GetTree(ctx, s.GetReadRepo(), s.GetTreeID())
	if err != nil {
		return &Response{StatusCode: http.StatusInternalServerError, Output: map[string]interface{}{"error": err.Error()}}, false
	}

	// now that we have the tree, look up the actual URI in that tree to find
	// the object UUID, then pull that from the repo
	requested, err := s.GetReadRepo().Find(ctx, tree.Tree[noHashPath])
	if err != nil {
		return &Response{StatusCode: http.StatusNotFound, Output: map[string]interface{}{"error": err.Error()}}, false
	}
	item, ok := requested.(*domain.RedfishResource)
	if !ok {
		return &Response{StatusCode: http.StatusInternalServerError, Output: map[string]interface{}{"error": errors.New("Expected a RedfishResource, but got something strange.")}}, false
	}

	// security privileges. Check to see if user has permissions on the object
	// FIXME/TODO: SIMPLE IMPLEMENTATION... this needs to handle AND/OR combinations.
	// Also need to consider purity. This could realistically be implemented as two additional Service wrappers:
	//  1) a pre-call check that does the gross privilege check
	//  2) a post-call check that filters the properties returned based on privs
	getPrivs, ok := item.PrivilegeMap[r.Method]
	if !ok {
		return &Response{StatusCode: http.StatusMethodNotAllowed, Output: map[string]interface{}{"error": err.Error()}}, false
	}

	getPrivsArr := getPrivs.([]string)

	fmt.Printf("CHECK PRIVS\n\tUSER: %s\n\tRESOURCE: %s\n", privileges, getPrivsArr)

	for _, myPriv := range privileges {
		for _, itemPriv := range getPrivsArr {
			if myPriv == itemPriv {
				fmt.Printf("Found matching privs, granting access. userPriv(%s) == itemPriv(%s)\n", myPriv, itemPriv)
				return nil, true
			}
		}
	}

    // User has been denied authorization. Now figure out which error code to return

    // 401 - Unauthorized: The authentication credentials included with this request are missing or invalid.
    // The "Invalid" case is handled in the respective basicauth or redfishxauthtoken classes earlier.
    //
    // Now, we check for "authorization-complete" privilege which would have been added earlier.
    // If it is MISSING, then raise 401 - Unauthorized
    // If it is PRESENT, then raise 403 - Forbidden
    deniedStatus := http.StatusUnauthorized
    for _,i := range(privileges) {
        if i == "authorization-complete" {
            deniedStatus = http.StatusForbidden
        }
    }

	return &Response{StatusCode: deniedStatus, Output: map[string]interface{}{"error": "Not authorized"}}, false
}



func (s *privilegeEnforcingService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (resp *Response, err error) {
    fmt.Printf("CheckAuthorization privileges h\n")
    response, authorized :=  s.IsAuthorized(ctx, r, privileges)

    if authorized {
        return s.Service.RedfishResourceHandler(ctx, r, privileges)
    }

    // not returning an error because for middleware purposes, there is no error that requires circuit breaking or other.
	return response, nil
}

func (s *privilegeEnforcingService) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (resp *Response, err error) {
    fmt.Printf("CheckAuthorization privileges\n")
    response, authorized :=  s.IsAuthorized(ctx, r, privileges)

    if authorized {
        fmt.Printf("\tCongratulations, you are authorized\n")
	    return s.Service.GetRedfishResource(ctx, r, privileges)
    }

    fmt.Printf("\tGo away: %s\n", response)
    // not returning an error because for middleware purposes, there is no error that requires circuit breaking or other.
	return response, nil
}
