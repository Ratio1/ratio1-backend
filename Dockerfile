FROM golang:1.17-alpine as builder

RUN mkdir -p /api
WORKDIR /api

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
WORKDIR ./cmd
RUN go build -o launchpad-api

WORKDIR /api
EXPOSE 5000 5000
ENTRYPOINT ["./cmd/launchpad-api"]
