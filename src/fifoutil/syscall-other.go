// +build !linux

package fifoutil

import (
	"errors"
)

func IsFIFO(path string) bool {
	return false
}

func MakeFifo(pipePath string, mode uint32) error {
	return errors.New("fifo not implemented in this OS")
}
