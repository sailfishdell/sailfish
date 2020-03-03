// +build !linux

package syscalls

import (
	"errors"
)

func MakeFifo(pipePath string, mode uint32) error {
	return errors.New("fifo not implemented in this OS")
}
