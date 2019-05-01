FROM python:3.7.0

ARG KUBECTL_VERSION=v1.12.0
ENV WORKSPACE /src/rancher-validation
WORKDIR $WORKSPACE
ENV PYTHONPATH /src/rancher-validation
ARG RKE_VERSION=v0.1.17

COPY [".", "$WORKSPACE"]

RUN wget https://storage.googleapis.com/kubernetes-release/release/$KUBECTL_VERSION/bin/linux/amd64/kubectl && \
    mv kubectl /bin/kubectl && \
    chmod +x /bin/kubectl  && \
    wget https://github.com/rancher/rke/releases/download/$RKE_VERSION/rke_linux-amd64 && \
    mv rke_linux-amd64 /bin/rke && \
    chmod +x /bin/rke && \
    cd $WORKSPACE && \
    pip install -r requirements_v3api.txt