package plugins

import (
	eh "github.com/looplab/eventhorizon"
)

type propgetter interface{
    GetProperty(string) interface{}
}

type propgetterunlocked interface{
    GetPropertyUnlocked(string) interface{}
}

// runtime panic if upper layers dont set properties for id/uri
func GetUUID(s propgetter) eh.UUID {
	return s.GetProperty("id").(eh.UUID)
}

func GetOdataID(s propgetter) string {
	return s.GetProperty("uri").(string)
}

func GetOdataIDUnlocked(s propgetterunlocked) string { 
	return s.GetPropertyUnlocked("uri").(string)
}
