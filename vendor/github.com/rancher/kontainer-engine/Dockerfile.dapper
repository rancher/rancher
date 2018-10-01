FROM golang:1.9.1
RUN apt-get update && \
    apt-get install -y xz-utils zip rsync
RUN go get github.com/rancher/trash
RUN go get golang.org/x/lint/golint
RUN curl -sL https://get.docker.com/builds/Linux/x86_64/docker-1.10.3 > /usr/bin/docker && \
    chmod +x /usr/bin/docker
ENV PATH /go/bin:$PATH
ENV DAPPER_SOURCE /go/src/github.com/rancher/kontainer-engine
ENV DAPPER_OUTPUT bin build/bin dist
ENV DAPPER_DOCKER_SOCKET true
ENV DAPPER_ENV TAG REPO GOOS CROSS
ENV GO15VENDOREXPERIMENT 1
ENV TRASH_CACHE ${DAPPER_SOURCE}/.trash-cache
WORKDIR ${DAPPER_SOURCE}
ENTRYPOINT ["./scripts/entry"]
CMD ["ci"]
