# Build Instructions

The base tag this release is branched from is `v2.4.3`


Create Environment Variables

```
export DOCKER_REPO=<Docker Repository>
export DOCKER_NAMESPACE=<Docker Namespace>
export DOCKER_TAG=<Image Tag>
```

Build and Push Images

```
# Build and push Rancher

make
docker tag rancher/rancher:dev ${DOCKER_REPO}/${DOCKER_NAMESPACE}/rancher:${DOCKER_TAG}
docker tag rancher/rancher-agent:dev ${DOCKER_REPO}/${DOCKER_NAMESPACE}/rancher-agent:${DOCKER_TAG}

docker push ${DOCKER_REPO}/${DOCKER_NAMESPACE}/rancher:${DOCKER_TAG}
docker push ${DOCKER_REPO}/${DOCKER_NAMESPACE}/rancher-agent:${DOCKER_TAG}

```