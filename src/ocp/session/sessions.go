package session

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
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

type cacheItem struct {
	token      string
	username   string
	privileges []string
	sessionuri string
}

func MakeHandlerFunc(logger log.Logger, eb eh.EventBus, getter IDGetter, withUser func(string, []string) http.Handler, chain http.Handler) http.HandlerFunc {
	tokenCache := []cacheItem{}
	tokenCacheMu := sync.RWMutex{}
	handlerlog := logger.New("module", "session")
	handlerlog.Crit("Creating x-auth-token based session handler.")
	return func(rw http.ResponseWriter, req *http.Request) {
		var userName string
		var privileges []string

		xauthtoken := req.Header.Get("X-Auth-Token")
		if xauthtoken != "" {
			// check cache before doing expensive parse
			foundCache := false
			tokenCacheMu.RLock()
			for _, i := range tokenCache {
				if i.token == xauthtoken && getter.HasAggregateID(i.sessionuri) {
					// comment out unused logs in the hotpath, uncomment to debug if needed
					//handlerlog.Debug("Cache Hit", "token", xauthtoken)
					userName = i.username
					privileges = i.privileges
					eb.PublishEvent(context.Background(), eh.NewEvent(XAuthTokenRefreshEvent, &XAuthTokenRefreshData{SessionURI: i.sessionuri}, time.Now()))
					foundCache = true
				}
			}
			tokenCacheMu.RUnlock()

			if !foundCache {
				token, err := jwt.ParseWithClaims(xauthtoken, &RedfishClaims{}, func(token *jwt.Token) (interface{}, error) {
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
					}
					return SECRET, nil
				})

				if err == nil && token != nil {
					if claims, ok := token.Claims.(*RedfishClaims); ok && token.Valid {
						if getter.HasAggregateID(claims.SessionURI) {
							userName = claims.UserName
							privileges = claims.Privileges
							eb.PublishEvent(context.Background(), eh.NewEvent(XAuthTokenRefreshEvent, &XAuthTokenRefreshData{SessionURI: claims.SessionURI}, time.Now()))

							tokenCacheMu.Lock()

							tokenCache = append(tokenCache, cacheItem{sessionuri: claims.SessionURI, username: userName, privileges: claims.Privileges, token: xauthtoken})
							if len(tokenCache) > 4 {
								//handlerlog.Debug("trim cache", "cache", tokenCache)
								tokenCache = tokenCache[1:]
							}

							tokenCacheMu.Unlock()
						}
					}
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

func SetupSessionService(svc *testaggregate.Service, v *view.View, d *domain.DomainObjects) {
	eh.RegisterCommand(func() eh.Command {
		return &POST{
			model:          v.GetModel("default"),
			commandHandler: d.CommandHandler,
			eventWaiter:    d.EventWaiter,
			svcWrapper: func(params map[string]interface{}) *view.View {
				_, vw, _ := svc.Instantiate("session", params)
				return vw
			},
		}
	})
}
