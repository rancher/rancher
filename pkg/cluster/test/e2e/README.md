### e2e tests

Right now, these tests assume there is at least one running k8s cluster to run against. So, they're currently skipped in CI. But, they are helpful for development and sanity checking.

Here's how to run them (from this directory):
```
TEST_CLUSTER_MGR_CONFIG=~/.kube/config TEST_CLUSTER_CONFIG=~/.kube/config go test -check.vv -v .
```
Obviously, the path to the kube-config file must be valid.
