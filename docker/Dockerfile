FROM golang:1.17 as builder
RUN apt update && \
    apt install make bash
ENV HOME /celestia-node
COPY / ${HOME}
WORKDIR ${HOME}
RUN make build 

FROM ubuntu
ENV NODE_TYPE full
ENV SLEEP 0s

WORKDIR $APP

# COPY docker/entrypoint.sh script/entrypoint.sh
COPY docker/entrypoint.sh /

# Copy in the binary
COPY --from=builder /celestia-node/celestia /

EXPOSE 2121

ENTRYPOINT ["/entrypoint.sh"]
CMD ["celestia"]