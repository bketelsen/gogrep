# syntax=docker/dockerfile:1.7

FROM golang:1.24-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

ARG VERSION=dev
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /out/gogrep .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/gogrep /usr/local/bin/gogrep
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/gogrep"]
