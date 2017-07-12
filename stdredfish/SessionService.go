package stdredfish

import (
	"context"
	"encoding/json"
	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/go-rfs/domain"
	"net/http"

	"fmt"
)

var _ = fmt.Println

type LoginRequest struct {
	UserName string
	Password string
}

func init() {
	domain.Httpsagas = append(domain.Httpsagas, SetupSessionService)
}

func SetupSessionService(s domain.SagaRegisterer, eventhandler eh.EventHandler) {
	s.RegisterNewSaga("POST:/redfish/v1/SessionService/Sessions",
		func(ctx context.Context, treeID, cmdID eh.UUID, resource *domain.RedfishResource, r *http.Request) error {
			decoder := json.NewDecoder(r.Body)
			var lr LoginRequest
			err := decoder.Decode(&lr)

			if err == nil {
				fmt.Printf("HAPPY: user(%s) pass(%s)\n", lr.UserName, lr.Password)
			}

			event := eh.NewEvent(domain.HTTPCmdProcessedEvent,
				&domain.HTTPCmdProcessedData{
					CommandID: cmdID,
					Results: map[string]interface{}{
						"MSG":  "HELLO WORLD SPECIAL",
						"user": lr.UserName,
						"pass": lr.Password,
					},
					Headers: map[string]string{
						"X-Token-Auth": "HELLO WORLD SPECIAL",
						"Location":     "/redfish/v1/SessionService/Sessions/1",
					},
				})

			eventhandler.HandleEvent(ctx, event)

			return nil
		})
}
