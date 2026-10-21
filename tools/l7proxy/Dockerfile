
# syntax=docker/dockerfile:1

FROM alpine:3.22@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1 AS ca-certificates
RUN apk add --no-cache 'ca-certificates>=20261020-r0'

FROM golang:1.26.10-alpine AS builder
WORKDIR /app
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /proxy .

FROM scratch
COPY --from=ca-certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /proxy /
EXPOSE 8080
ENV ALLOWED_HOSTS=
ENTRYPOINT ["/proxy"]