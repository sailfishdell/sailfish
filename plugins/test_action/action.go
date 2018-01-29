package test_action

import (
    "context"
    "time"
    "fmt"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/redfishresource"

    ah  "github.com/superchalupa/go-redfish/plugins/actionhandler"
)

func init() {
	domain.RegisterInitFN(InitService2)
	domain.RegisterInitFN(CreateTestActionEndpoint)
}


// Example of creating a minimal tree object to recieve action requests. Doesn't need much more
func CreateTestActionEndpoint(ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
    // Create SessionService aggregate
    ch.HandleCommand(
        context.Background(),
        &domain.CreateRedfishResource{
            ID:          eh.NewUUID(),
            ResourceURI: "/redfish/v1/Actions/Test",
            Type:        "Action",
            Context:     "Action",
            Plugin:      "GenericActionHandler",
            Privileges: map[string]interface{}{
                "POST":   []string{"ConfigureManager"},
            },
            Properties: map[string]interface{}{},
        },
    )
}

// Here is the actual service that does the heavy lifting for the actions
func InitService2(ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
    ctx := context.Background()

	l, err := ew.Listen(ctx, ah.MakeListener("/redfish/v1/Actions/Test"))
	if err != nil {
        fmt.Printf("Error creating listener: %s\n", err.Error())
		return
	}

    // never ending background process
    go func() {
		defer l.Close()

        for {
            select {
            case c := <-l.Inbox():
                // process action
                fmt.Printf("HAPPY\n")

                eb.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
                    CommandID:  c.Data().(ah.GenericActionEventData).CmdID,
                    Results:    map[string]interface{}{"happy": "joy"},
                    StatusCode: 200,
                    Headers:    map[string]string{},
                }, time.Now()))

                continue
            }
        }
    }()
}
