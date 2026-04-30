FROM hashicorp/nomad-autoscaler:0.4 AS autoscaler

FROM golang:1.26 AS builder

WORKDIR /build
COPY Makefile .
# do nodesim first, because it's a chonker and nice to cache
RUN make bin/nomad-nodesim

COPY go.* .
COPY plugins plugins
COPY observer observer
COPY holodeck holodeck
RUN make build

FROM golang:1.26

# for nodesim
RUN apt update && apt install -y \
    iptables \
    iproute2 \
    ;

WORKDIR /app
ENV PATH="/app/bin:${PATH}"
COPY --from=autoscaler /bin/nomad-autoscaler /app/bin/nomad-autoscaler
COPY --from=builder /build/bin /app/bin

COPY demo demo
ENTRYPOINT ["/app/demo/entrypoint.sh"]
CMD ["bash"]
