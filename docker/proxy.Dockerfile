# The egress proxy image (cmd/egress-proxy plus docker/egress-allowlist.txt).
# Build from the repository root: docker build -f docker/proxy.Dockerfile -t flow-eval-proxy .
FROM golang:1.27.2-trixie AS build
WORKDIR /src
COPY go.mod ./
COPY cmd/egress-proxy ./cmd/egress-proxy
RUN CGO_ENABLED=0 go build -trimpath -o /out/egress-proxy ./cmd/egress-proxy

FROM scratch
COPY --from=build /out/egress-proxy /egress-proxy
COPY docker/egress-allowlist.txt /etc/flow-eval/egress-allowlist.txt
USER 65534:65534
EXPOSE 3128
ENTRYPOINT ["/egress-proxy"]
