package stdredfish

import (
	"context"
	"encoding/json"
	"errors"
	jwt "github.com/dgrijalva/jwt-go"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/domain"
	"math/rand"
	"net/http"
	"time"

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

// This is a fairly slow implementation, but should be good enough for our
// purposes. This could be optimized to operate in about 1/5th of the time
const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

var moduleRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func createRandSecret(length int, characters string) []byte {
	b := make([]byte, length)
	charLen := len(characters)
	for i := range b {
		b[i] = characters[moduleRand.Intn(charLen)]
	}
	return b
}

func SetupSessionService(s domain.SagaRegisterer, d domain.DDDFunctions) {
	s.RegisterNewSaga("POST:/redfish/v1/SessionService/Sessions",
		func(ctx context.Context, treeID, cmdID eh.UUID, resource *domain.RedfishResource, r *http.Request) error {
			decoder := json.NewDecoder(r.Body)
			var lr LoginRequest
			err := decoder.Decode(&lr)

			if err != nil {
				return nil
			}

			privileges := []string{}
			account, err := domain.FindUser(ctx, d, lr.UserName)
			// TODO: verify password
			if err != nil {
				return errors.New("nonexistent user")
			}
			privileges = append(privileges, domain.GetPrivileges(ctx, d, account)...)

			uuid := eh.NewUUID()
			sessionURI := fmt.Sprintf("%s/v1/SessionService/Sessions/%s", d.GetBaseURI(), uuid)

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
				"Id":             fmt.Sprintf("%s", uuid),
				"Name":           "User Session",
				"Description":    "User Session",
				"UserName":       lr.UserName,
			}

			// we have the tree ID, fetch an updated copy of the actual tree
			tree, err := domain.GetTree(ctx, s.GetReadRepo(), treeID)
			if err != nil {
				return err
			}

			sessionServiceID, ok := tree.Tree[d.MakeFullyQualifiedV1("SessionService/Sessions")]
			if !ok {
				return errors.New("Couldn't get handle for session service")
			}

			err = s.GetCommandBus().HandleCommand(ctx, &domain.CreateRedfishResource{
				UUID:        uuid,
				ResourceURI: sessionURI,
				Type:        "#Session.v1_0_0.Session",
				Context:     d.MakeFullyQualifiedV1("$metadata#Session.Session"),
				Properties:  retprops,
				Private:     map[string]interface{}{"token_secret": secret},
			})
			if err != nil {
				return err
			}

			err = s.GetCommandBus().HandleCommand(ctx, &domain.AddRedfishResourceCollectionMember{UUID: sessionServiceID, MemberURI: sessionURI})
			if err != nil {
				return err
			}

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

			return nil
		})

	s.RegisterNewSaga("DELETE:/redfish/v1/$metadata#Session.Session",
		func(ctx context.Context, treeID, cmdID eh.UUID, resource *domain.RedfishResource, r *http.Request) error {
			// we have the tree ID, fetch an updated copy of the actual tree
			// TODO: Locking? Should repo give us a copy? Need to test this.
			tree, err := domain.GetTree(ctx, s.GetReadRepo(), treeID)
			if err != nil {
				return err
			}

			sessionID, ok := tree.Tree[r.URL.Path]
			if !ok {
				return errors.New("Couldn't get handle for session service")
			}

			s.GetCommandBus().HandleCommand(ctx, &domain.RemoveRedfishResource{UUID: sessionID})

			event := eh.NewEvent(domain.HTTPCmdProcessedEvent,
				&domain.HTTPCmdProcessedData{
					CommandID: cmdID,
					Results:   map[string]interface{}{"msg": "complete!"},
					Headers:   map[string]string{},
				})

			d.GetEventHandler().HandleEvent(ctx, event)

			return nil
		})
}
