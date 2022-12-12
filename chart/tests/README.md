
## local dev testing instructions

Option 1: Full chart CI run with a live cluster

```bash
./scripts/charts/ci 
```

Option 2: Test runs against the chart only 

```bash
# Install plugin if necessary, for version see CATTLE_HELM_UNITTEST_VERSION
# For a Mac you'll need to download a release and install the darwin binary
# For linux:
export ARCH=amd64 export CATTLE_HELM_UNITTEST_VERSION={version} helm plugin install https://github.com/rancher/helm-unittest

# change automated parts of templates
test_image="rancher/rancher"
test_image_tag="v2.7.0"
sed -i -e "s/%VERSION%/${test_image_tag}/g" ./chart/Chart.yaml
sed -i -e "s/%APP_VERSION%/${test_image_tag}/g" ./chart/Chart.yaml
sed -i -e "s@%POST_DELETE_IMAGE_NAME%@${test_image}@g" ./chart/values.yaml
sed -i -e "s/%POST_DELETE_IMAGE_TAG%/${test_image_tag}/g" ./chart/values.yaml

# test
helm lint ./chart
helm unittest ./chart

# clean
git checkout chart/*
```

