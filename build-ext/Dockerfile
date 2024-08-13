FROM golang:1.22 as tools
ENV CGO_ENABLED 0
ENV GOCACHE /.cache/go-build
ENV GOMODCACHE /.cache/mod
# HACK: Otherwise the build is not cached, no idea why
RUN touch /ok
WORKDIR /src

FROM --platform=amd64 tools as k3d-deps
ARG K3D_VERSION
RUN curl -sLf https://github.com/k3d-io/k3d/releases/download/${K3D_VERSION}/k3d-linux-amd64 > /usr/bin/k3d && \
    chmod +x /usr/bin/k3d
FROM scratch as k3d
COPY --from=k3d-deps /usr/bin/k3d /k3d

FROM scratch as apiserver-poc
COPY ext /usr/bin/apiserver-poc
ENTRYPOINT ["/usr/bin/apiserver-poc"]
