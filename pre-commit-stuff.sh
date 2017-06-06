#!/bin/sh

# list of tools
# https://dominik.honnef.co/posts/2014/12/go-tools
#
# go get https://github.com/golang/tools
# go get golang.org/cmd/gofmt
# go get github.com/golang/lint/golint
# go get github.com/kisielk/errcheck
# go get github.com/nsf/gocode
# go get code.google.com/p/rog-go/exp/cmd/godef
#   - vim-godef: https://github.com/dgryski/vim-godef
# go get golang.org/x/tools/cmd/gorename
# go get github.com/kisielk/godepgraph
# go get github.com/smartystreets/goconvey

go fmt  \
    github.com/superchalupa/go-redfish/commands/mockserver  \
    github.com/superchalupa/go-redfish/src/redfishserver/   \
    github.com/superchalupa/go-redfish/src/mockbackend/

go vet  \
    github.com/superchalupa/go-redfish/commands/mockserver  \
    github.com/superchalupa/go-redfish/src/redfishserver/   \
    github.com/superchalupa/go-redfish/src/mockbackend/

golint commands/mockserver/  src/mockbackend/  src/redfishserver/

errcheck  \
    github.com/superchalupa/go-redfish/commands/mockserver  \
    github.com/superchalupa/go-redfish/src/redfishserver/   \
    github.com/superchalupa/go-redfish/src/mockbackend/
