FROM docker.io/golang:1.20-alpine3.17 as builder

# hadolint ignore=DL3018
RUN apk update && apk add --no-cache \
    bash \
    gcc \
    git \
    make \
    musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN make build && make cel-key

FROM docker.io/alpine:3.18.0

# Read here why UID 10001: https://github.com/hexops/dockerfile/blob/main/README.md#do-not-use-a-uid-below-10000
ARG UID=10001
ARG USER_NAME=celestia

ENV CELESTIA_HOME=/home/${USER_NAME}

# Default node type can be overwritten in deployment manifest
ENV NODE_TYPE bridge
ENV P2P_NETWORK mocha

# hadolint ignore=DL3018
RUN apk update && apk add --no-cache \
        bash \
    # Creates a user with $UID and $GID=$UID
    && adduser ${USER_NAME} \
        -D \
        -g ${USER_NAME} \
        -h ${CELESTIA_HOME} \
        -s /sbin/nologin \
        -u ${UID}

# Copy in the binary
COPY --from=builder /src/build/celestia /bin/celestia
COPY --from=builder /src/./cel-key /bin/cel-key

COPY --chown=${USER_NAME}:${USER_NAME} docker/entrypoint.sh /opt/entrypoint.sh

USER ${USER_NAME}

EXPOSE 2121

ENTRYPOINT [ "/bin/bash", "/opt/entrypoint.sh" ]
CMD [ "celestia" ]
