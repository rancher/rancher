FROM ibuildthecloud/ubuntu-core-base:14.04
RUN apt-get update && apt-get install -y --no-install-recommends openjdk-7-jre-headless
RUN apt-get update && apt-get install -y curl
ENV CATTLE_HOME /var/lib/cattle
RUN mkdir -p /var/lib/cattle
ADD artifacts /usr/share/cattle
#RUN CATTLE_SSH_KEYGEN_ON_STARTUP=false /usr/share/cattle/cattle.sh --exit
CMD ["/usr/share/cattle/cattle.sh"]
EXPOSE 8080
ADD https://github.com/rancherio/cattle/releases/download/v0.5.0/cattle.jar /usr/share/cattle/
