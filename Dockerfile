FROM --platform=$BUILDPLATFORM golang:1.19 as builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG TARGETOS TARGETARCH
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

# FIXME -> ldflags are not working when using make
RUN echo "--> Building celestia" &&\
	go build -o build/ \
	-ldflags="-X 'main.buildTime=$(date)' -X 'main.lastCommit=$(git rev-parse HEAD)' -X 'main.semanticVersion=$(git describe --tags)'" \
	./cmd/celestia

RUN echo "--> Building cel-key" &&\
	go build -o build/ ./cmd/cel-key

FROM ubuntu:20.04
# Default node type can be overwritten in deployment manifest
ENV NODE_TYPE bridge
ENV P2P_NETWORK mocha

COPY docker/entrypoint.sh /

# Copy in the binary
COPY --from=builder /src/build/celestia /
COPY --from=builder /src/build/cel-key /

EXPOSE 2121

ENTRYPOINT ["/entrypoint.sh"]
CMD ["celestia"]
