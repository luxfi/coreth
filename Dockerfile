# ============= Compilation Stage ================
FROM golang:1.20.10-bullseye AS builder

ARG LUX_VERSION

RUN mkdir -p $GOPATH/src/github.com/luxdefi
WORKDIR $GOPATH/src/github.com/luxdefi

RUN git clone -b $LUX_VERSION --single-branch https://github.com/luxdefi/node.git

# Copy coreth repo into desired location
COPY . coreth

# Set the workdir to LuxGo and update coreth dependency to local version
WORKDIR $GOPATH/src/github.com/luxdefi/node
# Run go mod download here to improve caching of LuxGo specific depednencies
RUN go mod download
# Replace the coreth dependency
RUN go mod edit -replace github.com/luxdefi/coreth=../coreth
RUN go mod download && go mod tidy -compat=1.20

# Build the LuxGo binary with local version of coreth.
RUN ./scripts/build_avalanche.sh
# Create the plugins directory in the standard location so the build directory will be recognized
# as valid.
RUN mkdir build/plugins

# ============= Cleanup Stage ================
FROM debian:11-slim AS execution

# Maintain compatibility with previous images
RUN mkdir -p /node/build
WORKDIR /node/build

# Copy the executables into the container
COPY --from=builder /go/src/github.com/luxdefi/node/build .

CMD [ "./node" ]
