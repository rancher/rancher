FROM golang:1.5.3

RUN go get  github.com/golang/lint/golint \
            github.com/mattn/goveralls \
            golang.org/x/tools/cover \
            github.com/tools/godep \
            github.com/aktau/github-release

ENV USER root
WORKDIR /go/src/github.com/docker/machine

ADD . /go/src/github.com/docker/machine
RUN mkdir bin
