FROM ubuntu:22.04
COPY . /app
WORKDIR /app
EXPOSE 5000 22

RUN apt-get update -y && \
    apt-get install -y python3-pip python-dev-is-python3 build-essential curl dnsutils iputils-ping openssh-server net-tools && \
    mkdir /var/run/sshd && echo 'root:screencast' | chpasswd &&  \
    sed -i 's/PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    pip install -r requirements.txt

CMD ["/app/start.sh"]