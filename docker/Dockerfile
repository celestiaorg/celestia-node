FROM --platform=$BUILDPLATFORM golang:1.18 as builder
RUN apt-get install make
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN env GOOS=$TARGETOS GOARCH=$TARGETARCH make build

FROM ubuntu
# Default node type can be overwritten in deployment manifest
ENV NODE_TYPE bridge

COPY docker/entrypoint.sh /

# Copy in the binary
COPY --from=builder /src/build/celestia /

EXPOSE 2121

ENTRYPOINT ["/entrypoint.sh"]
CMD ["celestia"]