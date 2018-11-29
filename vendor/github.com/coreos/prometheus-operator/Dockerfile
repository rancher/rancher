FROM quay.io/prometheus/busybox:latest

ADD operator /bin/operator

# On busybox 'nobody' has uid `65534'
USER 65534

ENTRYPOINT ["/bin/operator"]
