FROM nginx
MAINTAINER Sangeetha Hariharan "https://github.com/sangeethah"

RUN apt-get update

RUN apt-get install -y wget
RUN apt-get install -y curl
RUN apt-get install -y iptables
RUN apt-get install -y dnsutils
RUN apt-get install -y iputils-ping

COPY ./run.sh /scripts/run.sh
RUN chmod 777 /scripts/run.sh


ENTRYPOINT [ "/scripts/run.sh" ]
CMD ["nginx", "-g", "daemon off;"]
