// +build linux

package triggers

import (
	"syscall"
)

func makeFifo(pipePath string, mode uint32) error {
	return syscall.Mkfifo(pipePath, mode)
}
