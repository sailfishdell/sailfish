package storage_volume

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#Volume.v1_0_3.Volume",
			Context:     "/redfish/v1/$metadata#Volume.Volume",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{"ConfigureManager"},
				"DELETE": []string{"ConfigureManager"},
			},

			//Need to add actions
			Properties: map[string]interface{}{
				"@Redfish.Settings": map[string]interface{}{ //Done
					"@odata.context": "/redfish/v1/$metadata#Settings.Settings",
					"@odata.id":      "/redfish/v1/Systems/System.Embedded.1/Storage/Volumes/$Entity/Settings",
					"@odata.type":    "#Settings.v1_1_0.Settings",
					"SupportedApplyTimes": []string{
						"Immediate",
						"OnReset",
						"AtMaintenanceWindowStart",
						"InMaintenanceWindowOnReset",
					},
				},
				"BlockSizeBytes@meta": v.Meta(view.PropGET("block_size")),  //Done
				"CapacityBytes@meta":  v.Meta(view.PropGET("capacity")),    //Done
				"Description@meta":    v.Meta(view.PropGET("description")), //Done
				"Encrypted@meta":      v.Meta(view.PropGET("encrypted")),   //DONE
				"EncryptionTypes":     []map[string]interface{}{},
				"Id@meta":             v.Meta(view.PropGET("unique_name")),
				"Identifiers":         []map[string]interface{}{},
				"Links": map[string]interface{}{
					"Drives": []map[string]interface{}{
						//Need to add Enclosures array
					},
					"Drives@odata.count": v.Meta(view.PropGET("count")),
				},
				"Name@meta": v.Meta(view.PropGET("name")), //Done
				"Oem": map[string]interface{}{ //Done
					"Dell": map[string]interface{}{
						"DellVirtualDisk": map[string]interface{}{
							"@odata.context": "/redfish/v1/$metadata#DellVirtualDisk.DellVirtualDisk",
							"@odata.id":      "/redfish/v1/Dell/Systems/System.Embedded.1/Storage/Volumes/DellVirtualDisk/$Entity",
							"@odata.type":    "#DellVirtualDisk.v1_0_0.DellVirtualDisk",

							"BusProtocol":         v.Meta(view.PropGET("bus_protocol")),
							"Cachecade":           v.Meta(view.PropGET("cache_cade")),
							"DiskCachePolicy":     v.Meta(view.PropGET("disk_cache_policy")),
							"LockStatus":          v.Meta(view.PropGET("lock_status")),
							"MediaType":           v.Meta(view.PropGET("media_type")),
							"ReadCachePolicy":     v.Meta(view.PropGET("read_cache_policy")),
							"SpanDepth":           v.Meta(view.PropGET("span_depth")),
							"SpanLength":          v.Meta(view.PropGET("span_length")),
							"VirtualDiskTargetID": v.Meta(view.PropGET("virtual_disk_target")),
							"WriteCachePolicy":    v.Meta(view.PropGET("write_cache_policy")),
						},
					},
				},
				"Operations": []map[string]interface{}{
					//Need to add Operations
				},
				"OptimumIOSizeBytes@meta": v.Meta(view.PropGET("optimum_io_size_bytes")),
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("obj_status")),
					"State@meta":        v.Meta(view.PropGET("state")),
					"Health@meta":       v.Meta(view.PropGET("obj_status")),
				},
				"VolumeType@meta": v.Meta(view.PropGET("volume_type")),
			}})
}
