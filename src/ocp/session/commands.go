package session

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

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

const (
	POSTCommand = eh.CommandType("SessionService:POST")
)

// HTTP POST Command
type POST struct {
	model          *model.Model
	commandHandler eh.CommandHandler
	eventWaiter    *eventwaiter.EventWaiter
	svcWrapper     func(map[string]interface{}) *view.View

	ID      eh.UUID           `json:"id"`
	CmdID   eh.UUID           `json:"cmdid"`
	Headers map[string]string `eh:"optional"`
	LR      LoginRequest
}

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&POST{})

func (c *POST) AggregateType() eh.AggregateType { return domain.AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) ParseHTTPRequest(r *http.Request) error {
	json.NewDecoder(r.Body).Decode(&c.LR)
	return nil
}
func (c *POST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	privileges := []string{} // no privs

	// hardcode some privileges for now
	// step 1: validate username/password (PUNT FOR NOW)
	// TODO: implement real pam here
	// TODO: Look up the privileges
	if c.LR.UserName == "Administrator" && c.LR.Password == "password" {
		privileges = append(privileges,
			"Unauthenticated", "tokenauth", "ConfigureSelf_"+c.LR.UserName,
			// TODO: Actually look up privileges
			"Login", "ConfigureManager", "ConfigureUsers", "ConfigureComponents",
		)
	} else if c.LR.UserName == "Operator" && c.LR.Password == "password" {
		privileges = append(privileges,
			"Unauthenticated", "tokenauth", "ConfigureSelf_"+c.LR.UserName,
			// TODO: Actually look up privileges
			"Login", "ConfigureComponents",
		)
	} else if c.LR.UserName == "ReadOnly" && c.LR.Password == "password" {
		privileges = append(privileges,
			"Unauthenticated", "tokenauth", "ConfigureSelf_"+c.LR.UserName,
			// TODO: Actually look up privileges
			"Login",
		)
	} else {
		return errors.New("could not verify username/password")
	}

	// instantiate here
	sessionVw := c.svcWrapper(map[string]interface{}{"username": c.LR.UserName})

	// step 2: Generate new session
	token := jwt.New(jwt.SigningMethodHS256)
	claims := make(jwt.MapClaims)
	//claims["exp"] = time.Now().Add(time.Hour * time.Duration(1)).Unix()
	claims["iat"] = time.Now().Unix()
	claims["iss"] = "localhost"
	claims["sub"] = c.LR.UserName
	claims["privileges"] = privileges
	claims["sessionuri"] = sessionVw.GetURI()
	token.Claims = claims
	secret := SECRET
	tokenString, _ := token.SignedString(secret)

	var timeout int
	switch t := c.model.GetProperty("session_timeout").(type) {
	case int:
		timeout = t
	case float64:
		timeout = int(t)
	case string:
		timeout, _ = strconv.Atoi(t)
	}

	// avoid recursive commands by running this in a goroutine. only important in cases of errors
	go c.startSessionDeleteTimer(sessionVw, timeout)

	a.PublishEvent(eh.NewEvent(domain.HTTPCmdProcessed, &domain.HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    nil, // TODO: return the uri contents
		StatusCode: 200,
		Headers: map[string]string{
			"X-Auth-Token": tokenString,
			"Location":     sessionVw.GetURI(),
		},
	}, time.Now()))
	return nil
}

func (c *POST) startSessionDeleteTimer(sessionVw *view.View, timeout int) {
	// all background stuff
	ctx := context.Background()
	sessionUUID := sessionVw.GetUUID()
	sessionURI := sessionVw.GetURI()
	logger := log.MustLogger("session")

	newCtx, cancel := context.WithCancel(ctx)
	duration := time.Duration(timeout) * time.Second
	timer := time.AfterFunc(duration, func() {
		// cancel the listener
		cancel()
		// delete the session
		c.commandHandler.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: sessionUUID, ResourceURI: sessionURI})
	})

	listener := eventwaiter.NewListener(newCtx, logger, c.eventWaiter, func(event eh.Event) bool {
		switch event.EventType() {
		case XAuthTokenRefreshEvent:
			if data, ok := event.Data().(*XAuthTokenRefreshData); ok {
				if data.SessionURI == sessionURI {
					// Got a token refresh, let ProcessEvents refresh the timer
					return true
				}
			}
		case domain.RedfishResourceRemoved:
			if data, ok := event.Data().(*domain.RedfishResourceRemovedData); ok {
				if data.ResourceURI == sessionURI {
					// Our session was deleted, we're done
					// cancel the context that will pop us out of ProcessEvents()
					cancel()

					// and stop the timer, no need to send the delete, it's already gone
					if !timer.Stop() {
						<-timer.C
					}
				}
			}
		}
		return false
	})

	defer listener.Close()

	listener.ProcessEvents(newCtx, func(ev eh.Event) {
		if !timer.Stop() {
			<-timer.C
		}
		timer.Reset(duration)
	})
}
