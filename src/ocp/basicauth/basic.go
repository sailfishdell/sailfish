package basicauth

import (
	"context"
	"net/http"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const (
	BasicAuthPlugin = domain.PluginType("obmc_basic_auth")
)

type Service struct {
	*plugins.Service
}

func New(options ...interface{}) (*Service, error) {
	s := &Service{
		Service: plugins.NewService(plugins.PluginType(BasicAuthPlugin)),
	}

	s.ApplyOption(plugins.UUID())
	s.ApplyOption(options...)
	return s, nil
}

func (a *Service) MakeHandlerFunc(withUser func(string, []string) http.Handler, chain http.Handler) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		username, password, ok := req.BasicAuth()
		privileges := []string{}
		if ok {
			// TODO: Actually look up privileges
			// hardcode some privileges for now
			if username == "Administrator" && password == "password" {
				privileges = append(privileges,
					"Unauthenticated", "basicauth", "ConfigureSelf_"+username,
					"Login", "ConfigureManager", "ConfigureUsers", "ConfigureComponents",
				)
			}
			if username == "Operator" && password == "password" {
				privileges = append(privileges,
					"Unauthenticated", "basicauth", "ConfigureSelf_"+username,
					"Login", "ConfigureComponents",
				)
			}
			if username == "ReadOnly" && password == "password" {
				privileges = append(privileges,
					"Unauthenticated", "basicauth", "ConfigureSelf_"+username,
					"Login",
				)
			}
		}
		if len(privileges) > 0 && username != "" {
			withUser(username, privileges).ServeHTTP(rw, req)
		} else {
			chain.ServeHTTP(rw, req)
		}
	}
}

func (s *Service) AddResource(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// no-op
}
