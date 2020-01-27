// +build linux

package udb

import (
	"syscall"
)

func makeFifo(pipePath string, mode uint32) error {
	return syscall.Mkfifo(pipePath, mode)
}
