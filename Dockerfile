#FROM  pgre-artifactory.pgre.dell.com:8001/golang:latest AS builder
FROM  golang:latest AS builder

# copy source tree in
COPY . /build/

# create a self-contained build structure
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' -mod=vendor -a -installsuffix nocgo -o /sailfish ./cmd/sailfish

FROM scratch
COPY --from=builder /sailfish  /
EXPOSE 443
ENTRYPOINT ["/sailfish"]
