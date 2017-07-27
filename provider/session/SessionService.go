package session

import (
	"context"
	"encoding/json"
	"errors"
	jwt "github.com/dgrijalva/jwt-go"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/domain"
	"github.com/superchalupa/go-redfish/provider"
	"math/rand"
	"net/http"
	"time"

	"fmt"
)

var _ = fmt.Println

//****************************************************************************
// Token Refresh Event handling
//  - token refresh is emitted by the http handler when we detect a valid token
//  on a request
//****************************************************************************
const (
	XAuthTokenRefreshEvent eh.EventType = "XAuthTokenRefresh"
)

type XAuthTokenRefreshData struct {
	SessionURI string
}

// End Token Refresh
//****************************************************************************

type LoginRequest struct {
	UserName string
	Password string
}

// This is a fairly slow implementation, but should be good enough for our
// purposes. This could be optimized to operate in about 1/5th of the time
const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz1234567890"

var moduleRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func createRandSecret(length int, characters string) []byte {
	b := make([]byte, length)
	charLen := len(characters)
	for i := range b {
		b[i] = characters[moduleRand.Intn(charLen)]
	}
	return b
}

func makeBackgroundDeleteSession(d domain.DDDFunctions, sessionURI string, sessionUUID eh.UUID) {
	go func() {
		// set up wait for token refresh
		refreshID, refreshChan := d.GetEventWaiter().SetupWait(func(event eh.Event) bool {
			if event.EventType() != XAuthTokenRefreshEvent {
				return false
			}
			if data, ok := event.Data().(*XAuthTokenRefreshData); ok {
				if data.SessionURI == sessionURI {
					return true
				}
			}
			return false
		})

		defer d.GetEventWaiter().CancelWait(refreshID)

		// if token is deleted by user, also stop
		deletedID, deletedChan := d.GetEventWaiter().SetupWait(func(event eh.Event) bool {
			if event.EventType() != domain.RedfishResourceRemovedEvent {
				return false
			}
			if event.AggregateID() == sessionUUID {
				return true
			}
			return false
		})

		defer d.GetEventWaiter().CancelWait(deletedID)

		// loop forever
		ctx := context.Background()
		for {
			select {

			case <-refreshChan:
				continue // stayin' alive

			case <-deletedChan:
				return // somebody else deleted it

			case <-time.After(30 * time.Second):
				// session times out, send command to delete
				d.GetCommandBus().HandleCommand(ctx, &domain.RemoveRedfishResource{RedfishResourceAggregateBaseCommand: domain.RedfishResourceAggregateBaseCommand{UUID: sessionUUID}})
				return //exit goroutine
			}
		}
	}()
}

func SetupSessionService(s provider.ProviderRegisterer) {
	eh.RegisterEventData(XAuthTokenRefreshEvent, func() eh.EventData { return &XAuthTokenRefreshData{} })

	s.RegisterHandler(
		"DELETE:context:"+s.GetBaseURI()+"/v1/$metadata#Session.Session",
		provider.RequestAdapter{Handle: provider.MakeStandardHTTPDelete(s)},
	)
	s.RegisterHandler(
		"POST:id:"+s.GetBaseURI()+"/v1/SessionService/Sessions",
		provider.RequestAdapter{Handle: makeHandlePost(s)},
	)
	return
}

func makeHandlePost(d domain.DDDFunctions) func(context.Context, *http.Request, []string, eh.UUID, *domain.RedfishTree, *domain.RedfishResource) error {
	return func(ctx context.Context, r *http.Request, privileges []string, cmdID eh.UUID, tree *domain.RedfishTree, requested *domain.RedfishResource) error {
		decoder := json.NewDecoder(r.Body)
		var lr LoginRequest
		err := decoder.Decode(&lr)

		if err != nil {
			return nil
		}

		account, err := domain.FindUser(ctx, d, lr.UserName)
		// TODO: verify password
		if err != nil {
			return errors.New("nonexistent user")
		}
		privileges = append(privileges, domain.GetPrivileges(ctx, d, account)...)

		sessionUUID := eh.NewUUID()
		sessionURI := fmt.Sprintf("%s/v1/SessionService/Sessions/%s", d.GetBaseURI(), sessionUUID)

		token := jwt.New(jwt.SigningMethodHS256)
		claims := make(jwt.MapClaims)
		//claims["exp"] = time.Now().Add(time.Hour * time.Duration(1)).Unix()
		claims["iat"] = time.Now().Unix()
		claims["iss"] = "localhost"
		claims["sub"] = lr.UserName
		claims["privileges"] = privileges
		claims["sessionuri"] = sessionURI
		token.Claims = claims
		secret := createRandSecret(24, characters)
		tokenString, err := token.SignedString(secret)

		retprops := map[string]interface{}{
			"@odata.type":    "#Session.v1_0_0.Session",
			"@odata.id":      sessionURI,
			"@odata.context": d.MakeFullyQualifiedV1("$metadata#Session.Session"),
			"Id":             fmt.Sprintf("%s", sessionUUID),
			"Name":           "User Session",
			"Description":    "User Session",
			"UserName":       lr.UserName,
		}

		err = d.GetCommandBus().HandleCommand(ctx, &domain.CreateRedfishResource{
			RedfishResourceAggregateBaseCommand: domain.RedfishResourceAggregateBaseCommand{UUID: sessionUUID},
			TreeID:      d.GetTreeID(),
			ResourceURI: retprops["@odata.id"].(string),
			Type:        retprops["@odata.type"].(string),
			Context:     retprops["@odata.context"].(string),
			Properties:  retprops,
			Private:     map[string]interface{}{"token_secret": secret},
		})
		if err != nil {
			return err
		}

		// set up the session redfish resource
		event := eh.NewEvent(domain.HTTPCmdProcessedEvent,
			&domain.HTTPCmdProcessedData{
				CommandID: cmdID,
				Results:   retprops,
				Headers: map[string]string{
					"X-Auth-Token": tokenString,
					"Location":     sessionURI,
				},
			})

		d.GetEventHandler().HandleEvent(ctx, event)

		// set up a goroutine that will delete the session resource when it
		// times out.
		// it would be much more efficient if we had one goroutine that
		// just looped over all the session entries but this is easier to
		// set up for now, and doesn't really prove to be terribly resource
		// intensive until we get hundreds of sessions
		makeBackgroundDeleteSession(d, sessionURI, sessionUUID)

		return nil
	}
}
