FROM rancher/dind:v0.3.0
COPY ./scripts/bootstrap /scripts/bootstrap
RUN /scripts/bootstrap
WORKDIR /source
