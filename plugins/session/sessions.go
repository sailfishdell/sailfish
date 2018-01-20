package session

import (
	"context"
	"fmt"
	"net/http"

	domain "github.com/superchalupa/go-redfish/redfishresource"

	jwt "github.com/dgrijalva/jwt-go"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

var SECRET []byte = []byte("happyhappyjoyjoy1234")

type AddUserDetails struct {
	OnUserDetails      func(userName string, privileges []string) http.Handler
	WithoutUserDetails http.Handler
}

type RedfishClaims struct {
	Privileges []string `json:"privileges"`
	SessionURI string   `json:"sessionuri"`
	jwt.StandardClaims
}

func (a *AddUserDetails) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var userName string
	var privileges []string

	xauthtoken := req.Header.Get("X-Auth-Token")
	if xauthtoken != "" {
		fmt.Printf("GOT A TOKEN\n")
		token, _ := jwt.ParseWithClaims(xauthtoken, &RedfishClaims{}, func(token *jwt.Token) (interface{}, error) { return []byte("foobar"), nil })

		if claims, ok := token.Claims.(*RedfishClaims); ok {
			fmt.Printf("Got a parsed token: %v\n", claims)
			if token.Valid {
				userName = "ROOT"
				privileges = append(privileges, "authorization-complete")
				privileges = append(privileges, claims.Privileges...)

				//domain.SendEvent(ctx, s, session.XAuthTokenRefreshEvent, &session.XAuthTokenRefreshData{SessionURI: claims.SessionURI})
				return
			}
		}
	}

	if userName != "" && len(privileges) > 0 {
		a.OnUserDetails(userName, privileges).ServeHTTP(rw, req)
	} else {
		a.WithoutUserDetails.ServeHTTP(rw, req)
	}
	return
}

func NewService(ctx context.Context, rootID eh.UUID, ew *utils.EventWaiter, ch eh.CommandHandler, eb eh.EventBus) (aud *AddUserDetails) {
	fmt.Printf("SetupSessionService\n")
	// setup module secret
	SECRET = createRandSecret(24, characters)

	// register our command
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, commandHandler: ch, eventWaiter: ew} })

	// set up the return value since we already know it
	aud = &AddUserDetails{}

	l, err := ew.Listen(ctx, func(event eh.Event) bool {
		if event.EventType() != domain.RedfishResourceCreated {
			return false
		}
		if data, ok := event.Data().(*domain.RedfishResourceCreatedData); ok {
			if data.ResourceURI == "/redfish/v1" {
				return true
			}
		}
		return false
	})
	if err != nil {
		return
	}

	// wait for the root object to be created, then enhance it. Oneshot for now.
	go func() {
		defer l.Close()

		_, err := l.Wait(ctx)
		if err != nil {
			fmt.Printf("Error waiting for event: %s\n", err.Error())
			return
		}

		// Create SessionService aggregate
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				ResourceURI: "/redfish/v1/SessionService",
				Type:        "#SessionService.v1_0_2.SessionService",
				Context:     "/redfish/v1/$metadata#SessionService.SessionService",
				Privileges: map[string]interface{}{
					"GET":    []string{"ConfigureManager"},
					"POST":   []string{"ConfigureManager"},
					"PUT":    []string{"ConfigureManager"},
					"PATCH":  []string{"ConfigureManager"},
					"DELETE": []string{},
				},
				Properties: map[string]interface{}{
					"Id":          "SessionService",
					"Name":        "Session Service",
					"Description": "Session Service",
					"Status": map[string]interface{}{
						"State":  "Enabled",
						"Health": "OK",
					},
					"ServiceEnabled": true,
					"SessionTimeout": 30,
					"Sessions": map[string]interface{}{
						"@odata.id": "/redfish/v1/SessionService/Sessions",
					},
				}})

		// Create Sessions Collection
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				Plugin:      "SessionService",
				ResourceURI: "/redfish/v1/SessionService/Sessions",
				Type:        "#SessionCollection.SessionCollection",
				Context:     "/redfish/v1/$metadata#SessionCollection.SessionCollection",
				Privileges: map[string]interface{}{
					"GET":    []string{"ConfigureManager"},
					"POST":   []string{"Unauthenticated"},
					"PUT":    []string{"ConfigureManager"},
					"PATCH":  []string{"ConfigureManager"},
					"DELETE": []string{"ConfigureSelf"},
				},
				Collection: true,
				Properties: map[string]interface{}{
					"Name":                "Session Collection",
					"Members@odata.count": 0,
					"Members":             []map[string]interface{}{},
				}})

		ch.HandleCommand(ctx,
			&domain.UpdateRedfishResourceProperties{
				ID: rootID,
				Properties: map[string]interface{}{
					"SessionService": map[string]interface{}{"@odata.id": "/redfish/v1/SessionService"},
					"Links":          map[string]interface{}{"Sessions": map[string]interface{}{"@odata.id": "/redfish/v1/SessionService/Sessions"}},
				},
			})
	}()

	return
}
