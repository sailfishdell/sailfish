package session

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	"github.com/superchalupa/sailfish/src/eventwaiter"
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
	handlerlog := logger.New("module", "session")
	handlerlog.Crit("Creating x-auth-token based session handler.")
	return func(rw http.ResponseWriter, req *http.Request) {
		var userName string
		var privileges []string

		xauthtoken := req.Header.Get("X-Auth-Token")
		if xauthtoken != "" {
			// check cache before doing expensive parse
			foundCache := false
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

			if !foundCache {
				token, err := jwt.ParseWithClaims(xauthtoken, &RedfishClaims{}, func(token *jwt.Token) (interface{}, error) {
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
					}
					return SECRET, nil
				})

				if err == nil && token != nil {
					if claims, ok := token.Claims.(*RedfishClaims); ok && token.Valid {
						if getter.HasAggregateID(claims.SessionURI) {
							userName = claims.UserName
							privileges = claims.Privileges
							eb.PublishEvent(context.Background(), eh.NewEvent(XAuthTokenRefreshEvent, &XAuthTokenRefreshData{SessionURI: claims.SessionURI}, time.Now()))

							//handlerlog.Debug("Add cache item", "token", xauthtoken)
							tokenCache = append(tokenCache, cacheItem{sessionuri: claims.SessionURI, username: userName, privileges: claims.Privileges, token: xauthtoken})
							if len(tokenCache) > 4 {
								//handlerlog.Debug("trim cache", "cache", tokenCache)
								tokenCache = tokenCache[1:]
							}
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

func SetupSessionService(ctx context.Context, svc *testaggregate.Service, v *view.View, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, ch eh.CommandHandler, eb eh.EventBus, masterparams map[string]interface{}) {
	// somewhat of a violation of how i want to structure all this, but it's the best option for now
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchEvent(XAuthTokenRefreshEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Session Service"))
	EventPublisher.AddObserver(EventWaiter)

	eh.RegisterCommand(func() eh.Command {
		return &POST{
			model:          v.GetModel("default"),
			commandHandler: ch,
			eventWaiter:    EventWaiter,
			svcWrapper: func(params map[string]interface{}) *view.View {

				newParams := map[string]interface{}{}
				for k, v := range masterparams {
					newParams[k] = v
				}
				for k, v := range params {
					newParams[k] = v
				}

				_, vw, _ := svc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "session", newParams)
				return vw
			},
		}
	})
}
