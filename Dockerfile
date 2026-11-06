# syntax=docker/dockerfile:1.7
ARG BASE_IMAGE=ghcr.io/below-z3r0/b3l0w-ort:cpu-1.31

FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go test ./... \
    && mkdir -p /out/bin /out/lib \
    && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/bin/aurisd ./cmd/aurisd \
    && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/bin/aurisctl ./cmd/aurisctl \
    && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/bin/voxd ./cmd/voxd \
    && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/bin/voxctl ./cmd/voxctl \
    && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/bin/streamd ./cmd/streamd \
    && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/bin/streamctl ./cmd/streamctl \
    && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/bin/sonus-mcp ./cmd/sonus-mcp \
    && cp -a /go/pkg/mod/github.com/k2-fsa/sherpa-onnx-go-linux/v2@v2.0.1/lib/x86_64-unknown-linux-gnu/. /out/lib/

FROM ${BASE_IMAGE}
ARG GIT_SHA=dev
LABEL org.opencontainers.image.source="https://github.com/Below-Z3R0/sonus"
LABEL org.opencontainers.image.revision=$GIT_SHA
COPY --from=build --chown=1000:1000 /out/bin/ /usr/local/bin/
COPY --from=build --chown=1000:1000 /out/lib/ /opt/sonus/lib/
ENV LD_LIBRARY_PATH=/opt/sonus/lib
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["voxd", "--addr", "0.0.0.0:9102"]
