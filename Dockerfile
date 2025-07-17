# ============= Compilation Stage ================
FROM golang:1.23.6-bullseye AS builder

ARG LUXD_VERSION

RUN mkdir -p $GOPATH/src/github.com/luxfi
WORKDIR $GOPATH/src/github.com/luxfi

RUN git clone -b $LUXD_VERSION --single-branch https://github.com/luxfi/node.git

# Copy geth repo into desired location
COPY . geth

# Set the workdir to Lux and update geth dependency to local version
WORKDIR $GOPATH/src/github.com/luxfi/node
# Run go mod download here to improve caching of Lux specific depednencies
RUN go mod download
# Replace the geth dependency
RUN go mod edit -replace github.com/luxfi/geth=../geth
RUN go mod download && go mod tidy -compat=1.23

# Build the Lux binary with local version of geth.
RUN ./scripts/build_lux.sh
# Create the plugins directory in the standard location so the build directory will be recognized
# as valid.
RUN mkdir build/plugins

# ============= Cleanup Stage ================
FROM debian:11-slim AS execution

# Maintain compatibility with previous images
RUN mkdir -p /node/build
WORKDIR /node/build

# Copy the executables into the container
COPY --from=builder /go/src/github.com/luxfi/node/build .

CMD [ "./node" ]
