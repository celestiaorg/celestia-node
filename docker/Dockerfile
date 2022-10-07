FROM --platform=$BUILDPLATFORM golang:1.19 as builder
RUN apt-get -y --no-install-recommends install make
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG TARGETOS TARGETARCH
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
RUN make build

FROM ubuntu:20.04
# Default node type can be overwritten in deployment manifest
ENV NODE_TYPE bridge

COPY docker/entrypoint.sh /

# Copy in the binary
COPY --from=builder /src/build/celestia /

EXPOSE 2121

ENTRYPOINT ["/entrypoint.sh"]
CMD ["celestia"]