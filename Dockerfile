# syntax=docker/dockerfile:1

##
## Build
##
FROM golang:1.18-buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /bin/ringleadr-reverse-proxy

##
## Deploy
##
FROM gcr.io/distroless/base-debian10

WORKDIR /

COPY --from=build /bin/ringleadr-reverse-proxy /bin/ringleadr-reverse-proxy

EXPOSE 14442

ENTRYPOINT ["/bin/ringleadr-reverse-proxy"]
