# syntax=docker/dockerfile:1
FROM docker.io/golang:1.26.8-alpine3.23 AS build

ARG VERSION=dev
ARG COMMIT=unknown

ENV CGO_ENABLED=0 GOWORK=off

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN mkdir -p build && \
    go build -ldflags "-X github.com/celestiaorg/celestia-node/nodebuilder/node.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/celestiaorg/celestia-node/nodebuilder/node.lastCommit=${COMMIT} -X github.com/celestiaorg/celestia-node/nodebuilder/node.semanticVersion=${VERSION}" \
      -o build/celestia ./cmd/celestia && \
    go build -o cel-key ./cmd/cel-key

FROM docker.io/alpine:3.24.1 AS runtime

ARG VERSION=dev
ARG COMMIT=unknown

LABEL org.opencontainers.image.source="https://github.com/celestiaorg/celestia-node" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}"

# hadolint ignore=DL3018
RUN apk add --no-cache bash ca-certificates coreutils curl jq
COPY docker/devnet/*.sh /opt/devnet/
ENV P2P_NETWORK=devnet

FROM runtime AS credentials
COPY --from=build /src/build/celestia /src/cel-key /usr/local/bin/
RUN bash /opt/devnet/credentials.sh

FROM runtime AS bridge
COPY --from=build /src/build/celestia /src/cel-key /usr/local/bin/
COPY --from=credentials /credentials /credentials
ENV CORE_HOST=validator CORE_RPC_PORT=26657 CORE_GRPC_PORT=9090
EXPOSE 2121 26658
HEALTHCHECK --interval=2s --timeout=5s --start-period=10s --retries=60 \
    CMD ["bash", "/opt/devnet/health.sh", "bridge"]
CMD ["bash", "/opt/devnet/bridge.sh"]
