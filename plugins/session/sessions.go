package session

import (
	"context"
	"fmt"
	"net/http"
	"time"

	plugins "github.com/superchalupa/go-redfish/plugins"
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
		a.OnUserDetails(userName, privileges).ServeHTTP(rw, req)
	} else {
		a.WithoutUserDetails.ServeHTTP(rw, req)
	}
	return
}

func NewService(eb eh.EventBus, g IDGetter) (aud *AddUserDetails) {
	// set up the return value since we already know it
	return &AddUserDetails{eb: eb, getter: g}
}

func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// setup module secret
	SECRET = createRandSecret(24, characters)

	// register our command
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, commandHandler: ch, eventWaiter: ew} })

	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURI("/redfish/v1"))
	if err == nil {
		sp.RunOnce(func(event eh.Event) {
			// wait for /redfish/v1 to be created, then pull out the rootid so that we can modify it
			rootID := event.Data().(domain.RedfishResourceCreatedData).ID
			AddSessionResource(ctx, rootID, ch)
		})
	}
}

func AddSessionResource(ctx context.Context, rootID eh.UUID, ch eh.CommandHandler) {
	// Create SessionService aggregate
	ch.HandleCommand(
		ctx,
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
}
