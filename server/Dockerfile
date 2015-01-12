FROM ubuntu:14.04.1
RUN apt-get update && apt-get install -y --no-install-recommends openjdk-7-jre-headless
RUN apt-get update && apt-get install -y curl
ENV CATTLE_HOME /var/lib/cattle
ADD artifacts /usr/share/cattle
VOLUME ["/var/lib/cattle"]
EXPOSE 8080
CMD ["/usr/share/cattle/cattle.sh"]
ADD https://github.com/rancherio/cattle/releases/download/v0.9.0/cattle.jar /usr/share/cattle/
