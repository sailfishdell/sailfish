package session

import (
	"context"
	"fmt"
	"net/http"
	"time"

	domain "github.com/superchalupa/go-redfish/redfishresource"

	jwt "github.com/dgrijalva/jwt-go"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func init() {
	domain.RegisterInitFN(InitService)
}

var SECRET []byte = []byte("happyhappyjoyjoy1234")

type IDGetter interface {
	HasAggregateID(string) bool
}

type AddUserDetails struct {
	eb                 eh.EventBus
	getter             IDGetter
	OnUserDetails      func(userName string, privileges []string) http.Handler
	WithoutUserDetails http.Handler
}

type RedfishClaims struct {
	UserName   string   `json:"sub"`
	Privileges []string `json:"privileges"`
	SessionURI string   `json:"sessionuri"`
	jwt.StandardClaims
}

func (a *AddUserDetails) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var userName string
	var privileges []string

	xauthtoken := req.Header.Get("X-Auth-Token")
	if xauthtoken != "" {
		token, _ := jwt.ParseWithClaims(xauthtoken, &RedfishClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}
			return SECRET, nil
		})

		if claims, ok := token.Claims.(*RedfishClaims); ok && token.Valid {
			if a.getter.HasAggregateID(claims.SessionURI) {
				userName = claims.UserName
				privileges = claims.Privileges
				a.eb.HandleEvent(context.Background(), eh.NewEvent(XAuthTokenRefreshEvent, XAuthTokenRefreshData{SessionURI: claims.SessionURI}, time.Now()))
			}
		}
	}

	if userName != "" && len(privileges) > 0 {
		fmt.Printf("Chain to next handler with user details from TOKEN.\n")
		a.OnUserDetails(userName, privileges).ServeHTTP(rw, req)
	} else {
		fmt.Printf("No token present, chain to next authentication method.\n")
		a.WithoutUserDetails.ServeHTTP(rw, req)
	}
	return
}

func NewService(eb eh.EventBus, g IDGetter) (aud *AddUserDetails) {
	// set up the return value since we already know it
	return &AddUserDetails{eb: eb, getter: g}
}

func InitService(ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// setup module secret
	SECRET = createRandSecret(24, characters)

	// background context to use
	ctx := context.Background()

	// register our command
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, commandHandler: ch, eventWaiter: ew} })

	// set up listener that will fire when it sees /redfish/v1 created
	l, err := ew.Listen(ctx, func(event eh.Event) bool {
		if event.EventType() != domain.RedfishResourceCreated {
			return false
		}
		if data, ok := event.Data().(domain.RedfishResourceCreatedData); ok {
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

		event, err := l.Wait(ctx)
		if err != nil {
			fmt.Printf("Error waiting for event: %s\n", err.Error())
			return
		}

		// wait for /redfish/v1 to be created, then pull out the rootid so that we can modify it
		rootID := event.Data().(domain.RedfishResourceCreatedData).ID

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
					"ServiceEnabled":      true,
					"SessionTimeout":      30,
					"SessionTimeout@meta": map[string]interface{}{"PATCH": map[string]interface{}{"plugin": "patch"}},
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
}
