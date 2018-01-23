# redfish server (in progress... not really redfishy yet!)

This server will eventually do redfish...

## Install/Compile

Prepare golang source structure and install necessary tools:

```
mkdir go
export GOPATH=$PWD/go
export PATH=$PATH:$GOPATH/bin
go get github.com/golang/dep/cmd/dep
```

Checkout go-redfish source and install dependencies:

```
mkdir -p go/src/github.com/superchalupa/
cd go/src/github.com/superchalupa
git clone https://github.com/superchalupa/go-redfish
cd go-redfish
dep ensure
```

## Usage

Run the backend:
```bash
go run cmd/ocp-server/*go
```

Visit https://localhost:8443/redfish/v1

To run the tests:
```bash
go test ./...
```
