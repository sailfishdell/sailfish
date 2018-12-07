package task_service


import (
    "context"
    "sync"
    "fmt"

    "github.com/superchalupa/sailfish/src/ocp/view"
    "github.com/superchalupa/sailfish/src/log"
    domain "github.com/superchalupa/sailfish/src/redfishresource"
    "github.com/superchalupa/sailfish/src/ocp/testaggregate"
    "github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
    "github.com/superchalupa/sailfish/src/ocp/model"

    eh "github.com/looplab/eventhorizon"
    "github.com/spf13/viper"
)

func InitTask(logger log.Logger, instantiateSvc *testaggregate.Service) {

    var syncModels func(m *model.Model, updates []model.Update)
    type newtask struct {
      uri string
      mdl *model.Model
    }
    newchan := make(chan newtask, 30)
    trigger := make(chan struct{})
    taskViews := map[string]*view.View{}

    //add system.chassis.1/attributes
    awesome_mapper2.AddFunction("add_attributes", func(args ...interface{}) (interface{}, error) {
      resourceURI, ok := args[0].(string)
      if !ok || resourceURI == "" {
        return false, nil
      }

      v, err := domain.InstantiatePlugin(domain.PluginType(resourceURI))
      if err != nil || v == nil {
        return false, nil
      }

      vw, ok := v.(*view.View)
      if !ok {
        return false, nil
      }

      mdl := vw.GetModel("default")
      if mdl == nil {
        return false, nil
      }

      mdl.AddObserver("task", syncModels)

      newchan <- newtask{resourceURI, mdl} //model is created, fire a notification

      return true, nil
    })

    syncModels = func(m *model.Model, updates []model.Update) { //whenever this model is updated, fire a notification
      select {
        case trigger <- struct{}{}:
        default:
      }
    }

    go func() {
      var attrModel *model.Model // model from syschas1/attr
      for {
        select {
        case <- trigger:
        case n := <- newchan:
          attrModel = n.mdl
          fmt.Println("new model from uri: ", n.uri)
          continue
        }

        //group index name
        attrModel.UnderRLock(func () {
          attributes, ok := attrModel.GetPropertyOkUnlocked("attributes")
          if !ok {
            return
          }

          attrMap := attributes.(map[string]map[string]map[string]interface{})

          taskMaps := []map[string]map[string]interface{}{attrMap["FWUpdateTask"], attrMap["ProfileTask"]}
          groups := []string{"FWUpdateTask", "ProfileTask"}

          // index, namemap := range taskMap

          for groupid, taskMap := range taskMaps {
            for index, namemap := range taskMap {
              id_raw, ok := namemap["Id"]
              if !ok {
                //this attribute not yet populated
                continue
              }
              id, ok := id_raw.(string)
              if !ok || id == "" || id == "unknown" {
                logger.Debug("Did not get task ID as a valid string")
                continue
              }

              name_raw, ok := namemap["Name"]
              if !ok {
                //this attribute not yet populated
                continue
              }
              name, ok := name_raw.(string)
              if !ok || name == "" {
                logger.Debug("Did not get task name as a valid string")
                continue
              }

              state_raw, ok := namemap["TaskState"]
              if !ok {
                //this attribute not yet populated
                continue
              }
              state, ok := state_raw.(string)
              if !ok || state == "" {
                logger.Debug("Did not get task state as a valid string")
                continue
              }

              status_raw, ok := namemap["TaskStatus"]
              if !ok {
                //this attribute not yet populated
                continue
              }
              status, ok := status_raw.(string)
              if !ok || status == "" {
                logger.Debug("Did not get task status as a valid string")
                continue
              }

              start_time_raw, ok := namemap["StartTime"]
              if !ok {
                //this attribute not yet populated
                continue
              }
              start_time, ok := start_time_raw.(string)
              if !ok || start_time == "" {
                logger.Debug("Did not get task start time as a valid string")
                continue
              }

              end_time_raw, ok := namemap["EndTime"]
              if !ok {
                //this attribute not yet populated
                continue
              }
              end_time, ok := end_time_raw.(string)
              if !ok || end_time == "" {
                logger.Debug("Did not get task end time as a valid string")
                continue
              }

              percent, ok := namemap["PercentComplete"]
              if !ok {
                //this attribute not yet populated
                continue
              }

              message_raw, ok := namemap["Message1"]
              if !ok {
                //this attribute not yet populated
                continue
              }
              message, ok := message_raw.(string)
              if !ok || message == "" {
                logger.Debug("Did not get task message as a valid string")
                continue
              }

              message_id_raw, ok := namemap["MessageID1"]
              if !ok {
                //this attribute not yet populated
                continue
              }
              message_id, ok := message_id_raw.(string)
              if !ok || message_id == "" {
                logger.Debug("Did not get task message ID as a valid string")
                continue
              }

              msg_args_1_raw, ok := namemap["MessageArg1-1"]
              if !ok {
                continue
              }
              msg_args_1, ok := msg_args_1_raw.(string)
              if !ok {
                logger.Debug("Did not get msg args 1 as a string")
                continue
              }

              msg_args_2_raw, ok := namemap["MessageArg1-2"]
              if !ok {
                continue
              }
              msg_args_2, ok := msg_args_2_raw.(string)
              if !ok {
                logger.Debug("Did not get msg args 2 as a string")
                continue
              }
              msg_args_3_raw, ok := namemap["MessageArg1-3"]
              if !ok {
                continue
              }
              msg_args_3, ok := msg_args_3_raw.(string)
              if !ok {
                logger.Debug("Did not get msg args 3 as a string")
                continue
              }

              var msg_args[]string
              if (msg_args_1 != "") {
                msg_args = append(msg_args, msg_args_1)
              }
              if (msg_args_2 != "") {
                msg_args = append(msg_args, msg_args_2)
              }
              if (msg_args_3 != "") {
                msg_args = append(msg_args, msg_args_3)
              }

              if _, ok := taskViews[id]; !ok {
                //instantiate and add to map of task views
                _, vw, _ := instantiateSvc.Instantiate("task", map[string]interface{}{
                  "task_id": id,
                  "task_msg_1_args": msg_args,
                  "task_name": name,
                  "task_state": state,
                  "task_status": status,
                  "task_start": start_time,
                  "task_end": end_time,
                  "task_percent": percent,
                  "task_msg_1": message,
                  "task_msg_1_id": message_id,
                  "Group": groups[groupid],
                  "Index": index,
                })
                fmt.Println("instantiating", vw.GetURI())
                taskViews[id] = vw
              }
            }
          }
        })
      }
    }()
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
                        "GET":    []string{"Login"},
                        "POST":   []string{}, // cannot create sub objects
                        "PUT":    []string{},
                        "PATCH":  []string{"ConfigureManager"},
                        "DELETE": []string{}, // can't be deleted
                    },
                    Properties: map[string]interface{}{
                        "Id":                       "TaskService",
                        "Name":                     "Task Service",
                        "Description":              "Represents the properties for the Task Service",
                        "Status": map[string]interface{}{
                          "State": "Enabled",
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
                        "GET":    []string{"Login"},
                        "POST":   []string{}, // cannot create sub objects
                        "PUT":    []string{},
                        "PATCH":  []string{"ConfigureManager"},
                        "DELETE": []string{}, // can't be deleted
                    },
                    Properties: map[string]interface{}{
                        "Name":                     "Task Collection",
                        "Description":              "Collection of Tasks",
                        "Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
                        "Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
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
                        "GET":    []string{"Login"},
                        "POST":   []string{}, // cannot create sub objects
                        "PUT":    []string{},
                        "PATCH":  []string{"ConfigureManager"},
                        "DELETE": []string{}, // can't be deleted
                    },
                    Properties: map[string]interface{}{
                        "Name@meta":                    vw.Meta(view.GETProperty("task_name")),
                        "Description":                  "Tasks running on EC are listed here",
                        "Id@meta":                      vw.Meta(view.GETProperty("task_id")),
                        "TaskState@meta":               vw.Meta(view.GETProperty("task_state")),
                        "TaskStatus@meta":              vw.Meta(view.GETProperty("task_status")),
                        "StartTime@meta":               vw.Meta(view.GETProperty("task_start")),
                        "EndTime@meta":                 vw.Meta(view.GETProperty("task_end")),
                        "PercentComplete@meta":         vw.Meta(view.GETProperty("task_percent")),
                        "Messages": []interface{}{
                          map[string]interface{}{
                            "Message@meta":                     vw.Meta(view.GETProperty("task_msg_1")),
                            "MessageArgs@meta":                 vw.Meta(view.GETProperty("task_msg_1_args")),
                            "MessageArgs@odata.count@meta":     vw.Meta(view.GETProperty("task_msg_1_args"), view.GETFormatter("count"), view.GETModel("default")),
                            "MessageId@meta":                   vw.Meta(view.GETProperty("task_msg_1_id")),
                          },
                        },
                        "Messages@odata.count":         1, //there is always exactly 1 message
                        //"Messages@meta":                vw.Meta(view.GETProperty("task_messages"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
                        //"Messages@odata.count@meta":    vw.Meta(view.GETProperty("task_messages"), view.GETFormatter("count"), view.GETModel("default")),
                    }},
            }, nil
    })

}
