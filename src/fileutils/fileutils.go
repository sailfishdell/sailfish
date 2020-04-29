package fileutils

import (
	"io"
	"os"
)

func FileExists(fn string) bool {
	fd, err := os.Stat(fn)
	if os.IsNotExist(err) {
		return false
	}
	return !fd.IsDir()
}

func Copy(from string, to string) bool {
	fromF, err := os.Open(from)
	if err != nil {
		return false
	}
	defer fromF.Close()

	toF, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return false
	}
	defer toF.Close()

	_, err = io.Copy(toF, fromF)
	if err != nil {
		return false
	}
	return true
}
