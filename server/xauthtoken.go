package redfishserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

    "github.com/superchalupa/go-rfs/domain"
	jwt "github.com/dgrijalva/jwt-go"
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

type RedfishClaims struct {
	Privileges []string `json:"privileges"`
	SessionURI string   `json:"sessionuri"`
	jwt.StandardClaims
}

func (s *xAuthTokenService) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	fmt.Printf("Looking for token\n")
	xauthtoken := r.Header.Get("X-Auth-Token")
	if xauthtoken != "" {
		fmt.Printf("GOT A TOKEN\n")
		token, _ := jwt.ParseWithClaims(xauthtoken, &RedfishClaims{}, func(token *jwt.Token) (interface{}, error) {
			fmt.Printf("Decoded token: %s\n", token)
			claims := token.Claims.(*RedfishClaims)
			fmt.Printf("SESSION URI: %s\n", claims.SessionURI)
			fmt.Printf("PRIVILEGES: %s\n", claims.Privileges)

			// we have the tree ID, fetch an updated copy of the actual tree
			tree, err := domain.GetTree(ctx, s.GetReadRepo(), s.GetTreeID())
			if err != nil {
                return nil, errors.New("couldnt get tree")
			}

			// now that we have the tree, look up the actual URI in that tree to find
			// the object UUID, then pull that from the repo
			requested, err := s.GetReadRepo().Find(ctx, tree.Tree[claims.SessionURI])
			if err != nil {
                return nil, errors.New("couldnt get session")
			}
			item, ok := requested.(*domain.RedfishResource)
			if !ok {
                return nil, errors.New("couldnt type assert item")
			}

			if secret, ok := item.Private["token_secret"]; ok {
				return []byte(secret.([]byte)), nil
			}
			return nil, errors.New("No session")
		})

        if claims, ok := token.Claims.(*RedfishClaims); ok {
            fmt.Printf("Got a parsed token: %v\n", claims)
            if token.Valid {
                fmt.Printf("It's valid!\n")
                privileges = append(privileges, claims.Privileges...)
            } else {
                fmt.Printf("It's INVALID, CANNOT USE!\n")
            }
        }
	}

	return s.Service.GetRedfishResource(ctx, r, privileges)
}

func (s *xAuthTokenService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	return s.Service.RedfishResourceHandler(ctx, r, privileges)
}
