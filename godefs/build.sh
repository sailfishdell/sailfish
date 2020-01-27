#!/bin/sh

cd $(dirname $0)

# gcc -o hello -I . bins/hello.c

go tool cgo -godefs ./fan.go > generated-fan.go
