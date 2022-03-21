FROM --platform=$BUILDPLATFORM golang:1.17 as builder
RUN apt-get update && \
    apt-get install make bash
ENV HOME /celestia-node
COPY / ${HOME}
WORKDIR ${HOME}
ARG TARGETPLATFORM
RUN env GOOS=linux GOARCH=TARGETPLATFORM make build

FROM ubuntu
# Default node type can be overwritten in deployment manifest
ENV NODE_TYPE bridge

COPY docker/entrypoint.sh /

# Copy in the binary
COPY --from=builder /celestia-node/celestia /

EXPOSE 2121

ENTRYPOINT ["/entrypoint.sh"]
CMD ["celestia"]