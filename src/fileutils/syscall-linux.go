// +build linux

package fileutils

import (
	"os"
	"syscall"
)

func IsFIFO(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.Mode().IsRegular() || info.IsDir() {
		return false
	}
	if info.Mode()&os.ModeNamedPipe == os.ModeNamedPipe {
		return true
	}
	return false
}

func MakeFifo(pipePath string, mode uint32) error {
	return syscall.Mkfifo(pipePath, mode)
}
