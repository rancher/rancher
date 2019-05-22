FROM python:3.7.0

ARG RKE_VERSION
ARG KUBECTL_VERSION=v1.9.0
ENV WORKSPACE /src/rancher-validation
WORKDIR $WORKSPACE

COPY [".", "$WORKSPACE"]

RUN wget https://github.com/rancher/rke/releases/download/$RKE_VERSION/rke_linux-amd64 && \
    wget https://storage.googleapis.com/kubernetes-release/release/$KUBECTL_VERSION/bin/linux/amd64/kubectl && \
    mv rke_linux-amd64 /bin/rke && \
    chmod +x /bin/rke  && \
    mv kubectl /bin/kubectl && \
    chmod +x /bin/kubectl  && \
    cd $WORKSPACE && \
    pip install -r requirements.txt