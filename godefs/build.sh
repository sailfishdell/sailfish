#!/bin/sh

cd $(dirname $0)

go tool cgo -godefs ./hello.go > generated-hello.go
