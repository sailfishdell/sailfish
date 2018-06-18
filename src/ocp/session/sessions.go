package session

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	jwt "github.com/dgrijalva/jwt-go"
	eh "github.com/looplab/eventhorizon"
)

var SECRET []byte = []byte("happyhappyjoyjoy1234")

type IDGetter interface {
	HasAggregateID(string) bool
}

const (
	SessionPlugin = domain.PluginType("obmc_session")
)

type RedfishClaims struct {
	UserName   string   `json:"sub"`
	Privileges []string `json:"privileges"`
	SessionURI string   `json:"sessionuri"`
	jwt.StandardClaims
}

func init() {
	// setup module secret
	SECRET = createRandSecret(24, characters)
}

func MakeHandlerFunc(eb eh.EventBus, getter IDGetter, withUser func(string, []string) http.Handler, chain http.Handler) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
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
				if getter.HasAggregateID(claims.SessionURI) {
					userName = claims.UserName
					privileges = claims.Privileges
					eb.PublishEvent(context.Background(), eh.NewEvent(XAuthTokenRefreshEvent, XAuthTokenRefreshData{SessionURI: claims.SessionURI}, time.Now()))
				}
			}
		}

		if userName != "" && len(privileges) > 0 {
			withUser(userName, privileges).ServeHTTP(rw, req)
		} else {
			chain.ServeHTTP(rw, req)
		}
		return
	}
}

func AddAggregate(ctx context.Context, v *view.View, rootID eh.UUID, ch eh.CommandHandler, eb eh.EventBus, ew waiter) {

	// somewhat of a violation of how i want to structure all this, but it's the best option for now
	eh.RegisterCommand(func() eh.Command { return &POST{model: v.GetModel("default"), commandHandler: ch, eventWaiter: ew} })

	// Create SessionService aggregate
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
			Type:        "#SessionService.v1_0_2.SessionService",
			Context:     "/redfish/v1/$metadata#SessionService.SessionService",
			Privileges: map[string]interface{}{
				"GET":    []string{"ConfigureManager"},
				"POST":   []string{},
				"PUT":    []string{},
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
				"SessionTimeout@meta": v.Meta(
					view.PropGET("session_timeout"),
					//					view.PropPATCH("session_timeout"),
				),
				"Sessions": map[string]interface{}{
					"@odata.id": "/redfish/v1/SessionService/Sessions",
				},
			}})

	// Create Sessions Collection
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  true,
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

	return
}
