FROM ubuntu:16.04
COPY . /app
WORKDIR /app
EXPOSE 5000 22

RUN apt-get update -y && \
    apt-get install -y python-pip python-dev build-essential curl dnsutils iputils-ping openssh-server net-tools && \
    mkdir /var/run/sshd && echo 'root:screencast' | chpasswd &&  \
    sed -i 's/PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    pip install -r requirements.txt

CMD ["/app/start.sh"]