FROM golang:1.26.10-alpine AS build
WORKDIR /app
COPY main.go ./
RUN go build -o /main main.go

FROM alpine:3.22@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1
RUN apk add --no-cache 'ca-certificates>=20261020-r0'
COPY --from=build /main /
ENTRYPOINT ["/main"]