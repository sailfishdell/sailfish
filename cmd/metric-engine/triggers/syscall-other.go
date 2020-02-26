// +build !linux

package triggers

import (
	"errors"
)

func makeFifo(pipePath string, mode uint32) error {
	return errors.New("fifo not implemented in this OS")
}
