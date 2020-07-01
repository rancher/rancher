# Build Instructions

The base tag this release is branched from is `v2.4.3`


Create Environment Variables

```
export DOCKER_REPO=<Docker Repository>
export DOCKER_NAMESPACE=<Docker Namespace>
export DOCKER_TAG=v2.4.3-OL
```

Build and Push Images

```
# Build and push Rancher

git tag -d v2.4.3 
git tag  v2.4.3 
make
docker tag rancher/rancher:v2.4.3  ${DOCKER_REPO}/${DOCKER_NAMESPACE}/rancher:${DOCKER_TAG}
docker tag rancher/rancher-agent:v2.4.3  ${DOCKER_REPO}/${DOCKER_NAMESPACE}/rancher-agent:${DOCKER_TAG}

docker push ${DOCKER_REPO}/${DOCKER_NAMESPACE}/rancher:${DOCKER_TAG}
docker push ${DOCKER_REPO}/${DOCKER_NAMESPACE}/rancher-agent:${DOCKER_TAG}

```
