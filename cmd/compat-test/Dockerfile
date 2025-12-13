FROM --platform=$BUILDPLATFORM docker.io/golang:1.25-alpine3.22 AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

ENV CGO_ENABLED=0
ENV GO111MODULE=on

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN export CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} && \
    go build -o /bin/celestia-node-compat-test ./cmd/compat-test

FROM docker.io/alpine:3.20.2

# Read here why UID 10001: https://github.com/hexops/dockerfile/blob/main/README.md#do-not-use-a-uid-below-10000
ARG UID=10001
ARG USER_NAME=compat-test

# hadolint ignore=DL3018
RUN apk update && apk add --no-cache \
    ca-certificates \
    && adduser ${USER_NAME} \
    -D \
    -g ${USER_NAME} \
    -h /home/${USER_NAME} \
    -s /sbin/nologin \
    -u ${UID}

USER ${USER_NAME}

COPY --from=builder --chown=${USER_NAME}:${USER_NAME} /bin/celestia-node-compat-test /bin/celestia-node-compat-test

ENTRYPOINT ["/bin/celestia-node-compat-test"]
