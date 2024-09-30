FROM golang:1.23.1 AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOSUMDB=off

WORKDIR /src

COPY . .

# build service
RUN go build -mod=vendor -a -tags netgo --installsuffix netgo -ldflags '-w' -o eth-validator-api ./cmd/

# build run container
FROM alpine:3.14
RUN apk --no-cache add \
    ca-certificates

RUN adduser -D -s /bin/sh app

COPY --from=builder /src/eth-validator-api /bin/eth-validator-api
RUN chmod a+x /bin/eth-validator-api

USER app