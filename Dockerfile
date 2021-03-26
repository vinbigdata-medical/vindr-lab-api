FROM golang:1.15.2 AS build


RUN mkdir /opt/app
WORKDIR /opt/app

COPY ./go.mod ./go.mod
RUN go mod download

COPY ./*.go ./
COPY ./account ./account
COPY ./annotation ./annotation
COPY ./constants ./constants 
COPY ./entities ./entities
COPY ./helper ./helper
COPY ./keycloak ./keycloak
COPY ./stats ./stats
COPY ./label_group ./label_group
COPY ./mw ./mw
COPY ./object ./object
COPY ./project ./project
COPY ./study ./study
COPY ./utils ./utils
COPY ./session ./session

RUN go build -o bin main.go

FROM debian:stable-slim

RUN mkdir /opt/app
WORKDIR  /opt/app

COPY --from=build /opt/app/bin /opt/app/bin
COPY ./templates ./templates
COPY ./mappings ./mappings

CMD ["./bin"]