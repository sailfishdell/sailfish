FROM golang:1.9 AS builder

# Download and install the latest release of dep
#ADD https://github.com/golang/dep/releases/download/v0.4.1/dep-linux-amd64 /usr/bin/dep
#RUN chmod +x /usr/bin/dep

# Copy the code from the host and compile it
#WORKDIR $GOPATH/src/github.com/superchalupa/go-redfish
#COPY Gopkg.toml Gopkg.lock ./
#RUN dep ensure --vendor-only

# copy source tree in
COPY . ./

# create a self-contained build structure
RUN rmdir src; \
     ln -s . src ;\
     ln -s . github.com  ;\
     ln -s . superchalupa    ;\
     ln -s . go-redfish     ;\
    GOPATH=$PWD CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o /app -tags simulation github.com/superchalupa/go-redfish/cmd/ocp-server

FROM scratch
COPY --from=builder /go/v1 /app  /
EXPOSE 443
ENTRYPOINT ["/app"]
