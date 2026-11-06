# syntax=docker/dockerfile:1.7
ARG BASE_IMAGE=ghcr.io/below-z3r0/b3l0w-ort:cpu-1.31

FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go test ./... \
    && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/aurisd ./cmd/aurisd \
    && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/aurisctl ./cmd/aurisctl \
    && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/voxd ./cmd/voxd \
    && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/voxctl ./cmd/voxctl

FROM ${BASE_IMAGE}
ARG GIT_SHA=dev
LABEL org.opencontainers.image.source="https://github.com/Below-Z3R0/sonus"
LABEL org.opencontainers.image.revision=$GIT_SHA
COPY --from=build --chown=1000:1000 /out/ /usr/local/bin/
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["voxd", "--addr", "0.0.0.0:9102"]
