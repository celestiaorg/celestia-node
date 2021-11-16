FROM golang:latest

RUN git clone https://github.com/celestiaorg/celestia-node.git

WORKDIR "/go/celestia-node"
RUN make install
RUN celestia full init
CMD celestia full start