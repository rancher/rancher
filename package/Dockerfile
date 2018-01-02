FROM ubuntu:16.04
RUN apt-get update && apt-get install -y git curl ca-certificates
RUN curl -sL https://github.com/rancher/machine-package/releases/download/v0.12.2/docker-machine.tar.gz | tar xvzf - -C /usr/bin
COPY rancher /usr/bin/
CMD ["rancher"]
