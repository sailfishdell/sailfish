#!/bin/sh

cd $(dirname $0)

gcc -o hello -I . bins/hello.c

go tool cgo -godefs ./hello.go > generated-hello.go
