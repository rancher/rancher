FROM ubuntu:14.04.2

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends openjdk-7-jre-headless \
                    curl \
                    mysql-server \
                    xz-utils \
                    keychain

ENV CATTLE_HOME /var/lib/cattle
ENV CATTLE_API_UI_INDEX http://releases.rancher.com/ui/0.19.2/static/index.html
ENV CATTLE_DB_CATTLE_DATABASE mysql
ADD artifacts /usr/share/cattle

ADD service /service
ENV S6_SERVICE_DIR /service

COPY target/*static.tar.gz /s6-statics/
RUN cd / && for i in $(ls /s6-statics/*static.tar.gz);do tar -zxvf $i;done

VOLUME ["/var/lib/cattle"]
VOLUME ["/var/lib/mysql"]

EXPOSE 8080
ENV RANCHER_SERVER_IMAGE v0.19.2

EXPOSE 3306
ADD https://github.com/rancherio/cattle/releases/download/v0.35.0/cattle.jar /usr/share/cattle/

ADD https://github.com/will-chan/machine/releases/download/v0.2.0-packet/docker-machine_linux-amd64 /usr/bin/docker-machine
RUN chmod +x /usr/bin/docker-machine

ADD https://github.com/rancherio/go-machine-service/releases/download/v0.8.0/go-machine-service.tar.xz /tmp/
RUN tar xpvf /tmp/go-machine-service.tar.xz -C /usr/bin/ && \
    chmod +x /usr/bin/go-machine-service

ENV CATTLE_MACHINE_EXECUTE true
CMD ["/usr/bin/s6-svscan", "/service"]
