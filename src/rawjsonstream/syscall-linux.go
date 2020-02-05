// +build linux

package rawjsonstream

import (
	"syscall"
)

func makeFifo(pipePath string, mode uint32) error {
	return syscall.Mkfifo(pipePath, mode)
}
