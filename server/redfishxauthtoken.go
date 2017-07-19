package redfishserver

import (
	"context"
	"errors"
	"fmt"
	jwt "github.com/dgrijalva/jwt-go"
	"net/http"

	"github.com/superchalupa/go-redfish/domain"
	"github.com/superchalupa/go-redfish/stdredfish"
)

var _ = fmt.Println

type xAuthTokenService struct {
	Service
}

// TODO:
//      -- in every request, reset timeout
//      -- if timeout passes, delete session

// NewXAuthTokenService returns a new instance of a xAuthToken Service.
func NewXAuthTokenService(s Service) Service {
	return &xAuthTokenService{Service: s}
}

type RedfishClaims struct {
	Privileges []string `json:"privileges"`
	SessionURI string   `json:"sessionuri"`
	jwt.StandardClaims
}

func (s *xAuthTokenService) CheckXAuthToken(ctx context.Context, r *http.Request) (resp *Response, privileges []string) {
	//fmt.Printf("Looking for token\n")
	xauthtoken := r.Header.Get("X-Auth-Token")
	if xauthtoken != "" {
		//fmt.Printf("GOT A TOKEN\n")
		token, err := jwt.ParseWithClaims(xauthtoken, &RedfishClaims{}, func(token *jwt.Token) (interface{}, error) {
			//fmt.Printf("Decoded token: %s\n", token)
			claims := token.Claims.(*RedfishClaims)
			//fmt.Printf("SESSION URI: %s\n", claims.SessionURI)
			//fmt.Printf("PRIVILEGES: %s\n", claims.Privileges)

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

		if err != nil {
			return &Response{StatusCode: http.StatusUnauthorized, Output: map[string]interface{}{"error": "X-Auth-Token parsing failed: " + err.Error()}}, nil
		}

		if claims, ok := token.Claims.(*RedfishClaims); ok {
			//fmt.Printf("Got a parsed token: %v\n", claims)
			if token.Valid {
				//fmt.Printf("It's valid!\n")
				privileges = []string{}
				privileges = append(privileges, "authorization-complete")
				privileges = append(privileges, claims.Privileges...)

				domain.SendEvent(ctx, s, stdredfish.XAuthTokenRefreshEvent, &stdredfish.XAuthTokenRefreshData{SessionURI: claims.SessionURI})

				return
			} else {
				return &Response{StatusCode: http.StatusUnauthorized, Output: map[string]interface{}{"error": "X-Auth-Token failed validation: "}}, nil
			}
		}
	}

	return
}

func (s *xAuthTokenService) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	response, basicAuthPrivs := s.CheckXAuthToken(ctx, r)
	if response != nil {
		return response, nil
	}

	if privileges != nil {
		privileges = append(privileges, basicAuthPrivs...)
	}
	return s.Service.GetRedfishResource(ctx, r, privileges)
}

func (s *xAuthTokenService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	response, basicAuthPrivs := s.CheckXAuthToken(ctx, r)
	if response != nil {
		return response, nil
	}

	if privileges != nil {
		privileges = append(privileges, basicAuthPrivs...)
	}
	return s.Service.RedfishResourceHandler(ctx, r, privileges)
}
