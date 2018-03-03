// Build tags: only build this for the openbmc build. Be sure to note the required blank line after.
// +build openbmc

package obmc

import (
	"time"
)

const (
   DbusTimeout time.Duration = 1
)

