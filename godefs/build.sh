#!/bin/sh

cd $(dirname $0)

gcc -o hello hello.c

go tool cgo -godefs ./hello.go > generated-hello.go
