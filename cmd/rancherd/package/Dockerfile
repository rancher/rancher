ARG ALPINE=alpine:3.11
FROM ${ALPINE} AS verify
ARG ARCH
ARG TAG
WORKDIR /verify
ADD https://github.com/rancher/rancher/releases/download/${TAG}/sha256sum.txt .
RUN set -x \
 && apk --no-cache add \
    curl \
    file
RUN export ARTIFACT="rancherd-amd64" \
 && curl --output ${ARTIFACT}  --fail --location https://github.com/rancher/rancher/releases/download/${TAG}/${ARTIFACT} \
 && grep "rancherd-amd64$" sha256sum.txt | sed "s/bin\///g"  | sha256sum -c \
 && mv -vf ${ARTIFACT} /opt/rancherd \
 && chmod +x /opt/rancherd \
 && file /opt/rancherd

RUN set -x \
 && apk --no-cache add curl \
 && curl -fsSLO https://storage.googleapis.com/kubernetes-release/release/v1.18.4/bin/linux/${ARCH}/kubectl \
 && chmod +x kubectl

FROM ${ALPINE}
ARG ARCH
ARG TAG
RUN apk --no-cache add \
    jq
COPY --from=verify /opt/rancherd /opt/rancherd
COPY scripts/upgrade.sh /bin/upgrade.sh
COPY --from=verify /verify/kubectl /usr/local/bin/kubectl
ENTRYPOINT ["/bin/upgrade.sh"]
CMD ["upgrade"]
