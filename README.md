# redfish server (in progress... almost Redfish OCP Profile compliant)

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
go run cmd/ocp-server/main.go -l https::8443 -l pprof:localhost:6060
```

Visit https://localhost:8443/redfish/v1

This starts up a golang profiling endpoint on port 6060 at /debug/pprof for local debugging as well

To run the (currently nonexistent) tests (need help here!):
```bash
go test ./...
```

## Where to start

A good place to start looking is plugins/obmc/bmc.go. This is where we set up the BMC manager object, and it's pretty self contained and understandable.

## Redfish Profile Validator

This server passes the redfish profile validation suite. To run that, check out the validtor and run it with the included config file, like so. (paths assume that service validator tool is checked out as a peer directory to go-redfish)
```bash
git clone https://github.com/DMTF/Redfish-Service-Validator
cd Redfish-Service-Validator
python3 ./RedfishServiceValidator.py -c ../go-redfish/scripts/Redfish-Service-Validator.ini 
```
