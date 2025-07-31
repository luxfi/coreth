# ============= Compilation Stage ================
FROM golang:1.23.6-bullseye AS builder

ARG LUX_VERSION

RUN mkdir -p $GOPATH/src/github.com/luxfi
WORKDIR $GOPATH/src/github.com/luxfi

RUN git clone -b $LUX_VERSION --single-branch https://github.com/luxfi/node.git

# Copy coreth repo into desired location
COPY . coreth

# Set the workdir to Luxd and update coreth dependency to local version
WORKDIR $GOPATH/src/github.com/luxfi/node
# Run go mod download here to improve caching of Luxd specific depednencies
RUN go mod download
# Replace the coreth dependency
RUN go mod edit -replace github.com/luxfi/coreth=../coreth
RUN go mod download && go mod tidy -compat=1.23

# Build the Luxd binary with local version of coreth.
RUN ./scripts/build_lux.sh
# Create the plugins directory in the standard location so the build directory will be recognized
# as valid.
RUN mkdir build/plugins

# ============= Cleanup Stage ================
FROM debian:11-slim AS execution

# Maintain compatibility with previous images
RUN mkdir -p /luxd/build
WORKDIR /luxd/build

# Copy the executables into the container
COPY --from=builder /go/src/github.com/luxfi/node/build .

CMD [ "./luxd" ]
