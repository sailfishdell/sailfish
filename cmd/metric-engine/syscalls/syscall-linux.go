// +build linux

package syscalls

import (
	"syscall"
)

func MakeFifo(pipePath string, mode uint32) error {
	return syscall.Mkfifo(pipePath, mode)
}
