# redfish server (in progress... almost Redfish OCP Profile compliant)

This server will eventually do redfish...

## Checkout and Dependencies

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
# Not really necessary any more:
# dep ensure
# the vendor/ directory is now included in the tree, so this step can be skipped
```

## Usage

First, you need to select which data source you'd like to compile for.
Currently "openbmc" is the only implemented (real) data source, but
"simulation" is under development and works. Add the tag for the backend you
want to the build, example below shows 'openbmc' build.

### Build and run the backend (OPENBMC build):
```bash
go build -tags openbmc github.com/superchalupa/go-redfish/cmd/ocp-server
./ocp-server -l https::8443 -l pprof:localhost:6060
```

Visit https://localhost:8443/redfish/v1


### Build and run the backend (SIMULATION build):
```bash
go build -tags simulation github.com/superchalupa/go-redfish/cmd/ocp-server
./ocp-server -l https::8443 -l pprof:localhost:6060
```

Visit https://localhost:8443/redfish/v1

This starts up a golang profiling endpoint on port 6060 at /debug/pprof for local debugging as well

### RUNNING the backend without building first

Please read the previous two sections about tags! Example is for a simulation run:

```bash
go run -tags "simulation" cmd/ocp-server/main.go cmd/ocp-server/without-spacemonkey.go -l https::8443 -l pprof:localhost:6060
```


### Tests

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

### Building for ARM with SPACEMONKEY (fast OpenSSL-based https support)

Golang implements it's own library for SSL/TLS. Unfortunately, this library is not as optimized as "openssl" on some systems. For instance, on ARM, golang takes twice as long to do SSL handshake compared to openssl. On most x86 and 64-bit ARM platforms, golang is competitive. So, for 32-bit ARM architectures, you should use spacemonkey, if possible, to get the best performance.

You can cross build *without* spacemonkey easily just by setting GOARCH=arm and running the go build tool. This will result in a binary that doesn't support the spacemonkey listener. Spacemonkey reduces the overhead by half for SSL-based HTTPS sessions on ARMv7.

You need to set up a few variables to point to your cross toolchain, and you'll need a cross go. Set the following variables. To run on aspeed or poleg, set GOARM=5, even though poleg is technically armv7

```bash
export PKG_CONFIG_PATH=.../path/to/cross/pkgconfig/pc/files/
export PATH=.../cross/go/bin:.../cross/gcc/

export GOARCH=arm
export GOARM=5
export GOOS=linux

go build -tags spacemonkey cmd/ocp-server/main.go
```

When running, activate the spacemonkey listener with the command line arg: -l spacemonkey:[addr]:[port]
