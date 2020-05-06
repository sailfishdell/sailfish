package task_service

import (
	"context"
	"strings"
	"sync"

	a "github.com/superchalupa/sailfish/src/dell-resources/attributedef"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
)

// helper
func getValidString(namemap, params map[string]interface{}, srcName, paramName string, invalid []string) bool {
	var ad a.AttributeData // for mapping the actual attribute date
	raw, ok := namemap[srcName]
	if !ok || !ad.Valid(raw) {
		return false
	}
	s, ok := ad.Value.(string)
	if !ok {
		return false
	}
	for _, bad := range invalid {
		if s == bad {
			return false
		}
	}

	params[paramName] = s
	return true
}

func InitTask(logger log.Logger, instantiateSvc *testaggregate.Service, am3Svc am3.Service, ch eh.CommandHandler, ctx context.Context) {

	//TODO: figure out what exactly updating the args should actually do
	/*awesome_mapper2.AddFunction("update_task_args", func (args ... interface{}) (interface{}, error) {
	  task_arg, ok := args[0].(string)
	  if !ok {
	    logger.Crit("Mapper configuration error: task arg not passed as string", "args[0]", args[0])
	    return nil, errors.New("Mapper configuration error: task arg not passed as string")
	  }
	  if task_arg == "" {
	    return nil, nil
	  }
	  task_arg_name, _ := args[1].(string) //this should always be a string
	  attributes, ok := attrModel.GetPropertyOkUnlocked("task_msg_1_args")
	}*/

	var syncModels func(m *model.Model, updates []model.Update)
	type newtask struct {
		uri string
		mdl *model.Model
	}
	newchan := make(chan newtask, 30)
	trigger := make(chan struct{})

	// TODO: (from MEB, code review note):
	// 			Not sure about the design here. need more comments. Looks like we are watching a single model at a time?

	//add system.chassis.1/attributes
	am3Svc.AddEventHandler("add_attributes", domain.RedfishResourceCreated, func(event eh.Event) {
		data, ok := event.Data().(*domain.RedfishResourceCreatedData)
		if !ok {
			logger.Error("Redfish Resource Created event did not match", "type", event.EventType, "data", event.Data())
			return
		}

		resourceURI := data.ResourceURI
		if resourceURI != "/redfish/v1/Chassis/System.Chassis.1/Attributes" {
			return
		}

		v, err := domain.InstantiatePlugin(domain.PluginType(resourceURI))
		if err != nil || v == nil {
			return
		}

		vw, ok := v.(*view.View)
		if !ok {
			return
		}

		mdl := vw.GetModel("default")
		if mdl == nil {
			return
		}

		mdl.AddObserver("task", syncModels)

		newchan <- newtask{resourceURI, mdl} //model is created, fire a notification
	})

	syncModels = func(m *model.Model, updates []model.Update) { //whenever this model is updated, fire a notification
		select {
		case trigger <- struct{}{}:
		default:
		}
	}

	go func() {
		createdTasks := map[string]bool{}

		var attrModel *model.Model // model from syschas1/attr
		var ad a.AttributeData     // for mapping the actual attribute date
		for {
			select {
			case <-trigger:
			case n := <-newchan:
				attrModel = n.mdl
				continue
			}

			//group index name

			// For the UnderRLock() code, DO AS LITTLE WORK AS POSSIBLE or we risk deadlocks
			// in particular, instantiate() is *terrible* as it touches everything
			// gather the data and do those things outside the lock
			instantiateList := []map[string]interface{}{}

			attrModel.UnderRLock(func() {
				attributes, ok := attrModel.GetPropertyOkUnlocked("attributes")
				if !ok {
					return
				}

				attrMap := attributes.(map[string]map[string]map[string]interface{})

				taskMaps := []map[string]map[string]interface{}{attrMap["FWUpdateTask"], attrMap["ProfileTask"]}
				groups := []string{"FWUpdateTask", "ProfileTask"}

				// index, namemap := range taskMap

				// don't allow a task to be created until it has all of its properties populated
				for groupid, taskMap := range taskMaps {
				inner:
					for index, namemap := range taskMap {
						params := map[string]interface{}{}
						params["Group"] = groups[groupid]
						params["Index"] = index
						params["task_state"] = ""

						for _, v := range []struct {
							src     string
							dst     string
							invalid []string
						}{
							{"Id", "task_id", []string{"", "unknown"}},
							{"Name", "task_name", []string{""}},
							{"TaskState", "STATE", []string{""}},
							{"TaskStatus", "task_status", []string{""}},
							{"StartTime", "task_start", []string{""}},
							{"EndTime", "task_end", []string{""}},
							{"Message1", "task_msg_1", []string{""}},
							{"MessageID1", "task_msg_1_id", []string{""}},
							{"MessageArg1-1", "a1", []string{}},
							{"MessageArg1-2", "a2", []string{}},
							{"MessageArg1-3", "a3", []string{}},
						} {
							if !getValidString(namemap, params, v.src, v.dst, v.invalid) {
								logger.Debug("Failed to parse valid string", "src", v.src, "dst", v.dst)
								continue inner
							}
							// early out for tasks we already have
							if _, ok := createdTasks[params["task_id"].(string)]; ok {
								continue inner
							}
						}

						if strings.EqualFold(params["STATE"].(string), "Completed") {
							params["STATE"] = "Completed"
						} else if strings.EqualFold(params["STATE"].(string), "Interrupted") {
							params["STATE"] = "Interrupted"
						} else {
							params["STATE"] = "Running"
						}

						percent_raw, ok := namemap["PercentComplete"]
						if !ok || !ad.Valid(percent_raw) {
							//this attribute not yet populated
							continue
						}
						params["task_percent"] = ad.Value

						var msg_args []string
						for _, msg := range []string{"a1", "a2", "a3"} {
							if params[msg].(string) != "" {
								msg_args = append(msg_args, msg)
							}
						}
						params["task_msg_1_args"] = msg_args

						// Mark that we have the task now so that it will meet early out
						// condition above and not attempt another instantiate.
						// Ideally, this would would be in instantiateTasksInList() after
						// confirming that the task is successfully instantiated.
						createdTasks[params["task_id"].(string)] = true

						// Add it to the pile to instantiate, then do it outside the lock
						instantiateList = append(instantiateList, params)
					}
				}
			})

			// Start a seperate GO routine to perform all the instantiations needed.
			// An issue was discovered where instantiate would take a while and if
			// a new task was created and changed too quickly, it would be completely
			// missed. This way, processing new tasks is not blocked.
			go instantiateTasksInList(logger, instantiateSvc, ctx, ch, instantiateList)
		}
	}()
}

//////////////////////////////////////////////////////////////////////
// Create a URI for each task defined in instantiateList.
// The instantiateList contains a map per URI with the intended fields.
//////////////////////////////////////////////////////////////////////
func instantiateTasksInList(logger log.Logger, instantiateSvc *testaggregate.Service, ctx context.Context, ch eh.CommandHandler, instantiateList []map[string]interface{}) {
	for _, params := range instantiateList {
		// Instantiate each task using the values in the given map (params).
		// NOTE: params is EXPECTED to have "task_id" and "STATE" keys if it made it here.
		_, vw, err := instantiateSvc.Instantiate("task", params)

		if err != nil {
			logger.Crit(params["task_id"].(string) + " task_service failed to instantiate: " + err.Error())
		} else {
			logger.Debug(params["task_id"].(string) + " task_service instantiated")

			// Add newly created URI to be handled
			ch.HandleCommand(ctx,
				&domain.UpdateRedfishResourceProperties2{
					ID:         vw.GetUUID(),
					Properties: map[string]interface{}{"TaskState": params["STATE"]},
				},
			)
		}
	}
}

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("task_service",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ID:          vw.GetUUID(),
					ResourceURI: vw.GetURI(),
					Type:        "#TaskService.v1_0_0.TaskService",
					Context:     params["rooturi"].(string) + "/$metadata#TaskService.TaskService",
					Privileges: map[string]interface{}{
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Id":          "TaskService",
						"Name":        "Task Service",
						"Description": "Represents the properties for the Task Service",
						"Status": map[string]interface{}{
							"State":  "Enabled",
							"Health": "OK",
						},

						"Tasks": map[string]interface{}{
							"@odata.id": vw.GetURI() + "/Tasks",
						},
					}},

				&domain.UpdateRedfishResourceProperties{
					ID: params["rootid"].(eh.UUID),
					Properties: map[string]interface{}{
						"TaskService": map[string]interface{}{"@odata.id": vw.GetURI()},
					},
				},
			}, nil
		})

	s.RegisterAggregateFunction("task_service_tasks",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ID:          vw.GetUUID(),
					ResourceURI: vw.GetURI(),
					Type:        "#TaskCollection.TaskCollection",
					Context:     params["rooturi"].(string) + "/$metadata#TaskCollection.TaskCollection",
					Privileges: map[string]interface{}{
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Name":                "Task Collection",
						"Description":         "Collection of Tasks",
						"Members":             []interface{}{},
						"Members@odata.count": 0,
					}},
			}, nil
		})

	s.RegisterAggregateFunction("task",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ID:          vw.GetUUID(),
					ResourceURI: vw.GetURI(),
					Type:        "#Task.v1_0_2.Task",
					Context:     params["rooturi"].(string) + "/$metadata#Task.Task",
					Privileges: map[string]interface{}{
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Name@meta":            vw.Meta(view.GETProperty("task_name")),
						"Description":          "Tasks running on EC are listed here",
						"Id@meta":              vw.Meta(view.GETProperty("task_id")),
						"TaskState":            "",
						"TaskStatus@meta":      vw.Meta(view.GETProperty("task_status")),
						"StartTime@meta":       vw.Meta(view.GETProperty("task_start")),
						"EndTime@meta":         vw.Meta(view.GETProperty("task_end")),
						"PercentComplete@meta": vw.Meta(view.GETProperty("task_percent")),
						"Messages": []interface{}{
							map[string]interface{}{
								"Message@meta":                 vw.Meta(view.GETProperty("task_msg_1")),
								"MessageArgs@meta":             vw.Meta(view.GETProperty("task_msg_1_args")),
								"MessageArgs@odata.count@meta": vw.Meta(view.GETProperty("task_msg_1_args"), view.GETFormatter("count"), view.GETModel("default")),
								"MessageId@meta":               vw.Meta(view.GETProperty("task_msg_1_id")),
							},
						},
						"Messages@odata.count": 1, //there is always exactly 1 message
						//"Messages@meta":                vw.Meta(view.GETProperty("task_messages"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						//"Messages@odata.count@meta":    vw.Meta(view.GETProperty("task_messages"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

}
