# Prometheus Operator
[![Build Status](https://travis-ci.org/coreos/prometheus-operator.svg?branch=master)](https://travis-ci.org/coreos/prometheus-operator)
[![Go Report Card](https://goreportcard.com/badge/coreos/prometheus-operator "Go Report Card")](https://goreportcard.com/report/coreos/prometheus-operator)

**Project status: *beta*** Not all planned features are completed. The API, spec, status and other user facing objects may change, but in a backward compatible way.

The Prometheus Operator for Kubernetes provides easy monitoring definitions for Kubernetes
services and deployment and management of Prometheus instances.

Once installed, the Prometheus Operator provides the following features:

* **Create/Destroy**: Easily launch a Prometheus instance for your Kubernetes namespace,
  a specific application or team easily using the Operator.

* **Simple Configuration**: Configure the fundamentals of Prometheus like versions, persistence, 
  retention policies, and replicas from a native Kubernetes resource.

* **Target Services via Labels**: Automatically generate monitoring target configurations based
  on familiar Kubernetes label queries; no need to learn a Prometheus specific configuration language.

For an introduction to the Prometheus Operator, see the initial [blog
post](https://coreos.com/blog/the-prometheus-operator.html).

## Prometheus Operator vs. kube-prometheus

The Prometheus Operator makes the Prometheus configuration Kubernetes native
and manages and operates Prometheus and Alertmanager clusters. It is a piece of
the puzzle regarding full end-to-end monitoring.

[kube-prometheus](contrib/kube-prometheus) combines the Prometheus Operator
with a collection of manifests to help getting started with monitoring
Kubernetes itself and applications running on top of it.

## Prerequisites

Version `>=0.18.0` of the Prometheus Operator requires a Kubernetes
cluster of version `>=1.8.0`. If you are just starting out with the
Prometheus Operator, it is highly recommended to use the latest version.

If you have an older version of Kubernetes and the Prometheus Operator running,
we recommend upgrading Kubernetes first and then the Prometheus Operator.

## CustomResourceDefinitions

The Operator acts on the following [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions/):

* **`Prometheus`**, which defines a desired Prometheus deployment.
  The Operator ensures at all times that a deployment matching the resource definition is running.

* **`ServiceMonitor`**, which declaratively specifies how groups
  of services should be monitored. The Operator automatically generates Prometheus scrape configuration
  based on the definition.

* **`PrometheusRule`**, which defines a desired Prometheus rule file, which can
  be loaded by a Prometheus instance containing Prometheus alerting and
  recording rules.

* **`Alertmanager`**, which defines a desired Alertmanager deployment.
  The Operator ensures at all times that a deployment matching the resource definition is running.

To learn more about the CRDs introduced by the Prometheus Operator have a look
at the [design doc](Documentation/design.md).

## Installation

Install the Operator inside a cluster by running the following command:

```sh
kubectl apply -f bundle.yaml
```

> Note: make sure to adapt the namespace in the ClusterRoleBinding if deploying in another namespace than the default namespace.

To run the Operator outside of a cluster:

```sh
make
hack/run-external.sh <kubectl cluster name>
```

## Removal

To remove the operator and Prometheus, first delete any custom resources you created in each namespace. The
operator will automatically shut down and remove Prometheus and Alertmanager pods, and associated configmaps.

```sh
for n in $(kubectl get namespaces -o jsonpath={..metadata.name}); do
  kubectl delete --all --namespace=$n prometheus,servicemonitor,alertmanager
done
```

After a couple of minutes you can go ahead and remove the operator itself.

```sh
kubectl delete -f bundle.yaml
```

The operator automatically creates services in each namespace where you created a Prometheus or Alertmanager resources,
and defines three custom resource definitions. You can clean these up now.

```sh
for n in $(kubectl get namespaces -o jsonpath={..metadata.name}); do
  kubectl delete --ignore-not-found --namespace=$n service prometheus-operated alertmanager-operated
done

kubectl delete --ignore-not-found customresourcedefinitions \
  prometheuses.monitoring.coreos.com \
  servicemonitors.monitoring.coreos.com \
  alertmanagers.monitoring.coreos.com
```

## Development

### Prerequisites

- golang environment
- docker (used for creating container images, etc.)
- minikube (optional)

### Testing

> Ensure that you're running tests in the following path:
> `$GOPATH/src/github.com/coreos/prometheus-operator` as tests expect paths to
> match. If you're working from a fork, just add the forked repo as a remote and
> pull against your local coreos checkout before running tests.

#### Running *unit tests*:

`make test-unit`

#### Running *end-to-end* tests on local minikube cluster:

1. `minikube start --kubernetes-version=v1.10.0 --memory=4096
    --extra-config=apiserver.Authorization.Mode=RBAC`
2. `eval $(minikube docker-env) && make image` - build Prometheus Operator
    docker image on minikube's docker
3. `make test-e2e`

## Contributing

Many files (documentation, manifests, ...) in this repository are
auto-generated. E.g. `bundle.yaml` originates from the _Jsonnet_ files in
`/jsonnet/prometheus-operator`. Before proposing a pull request:

1. Commit your changes.
2. Run `make generate-in-docker`.
3. Commit the generated changes.


## Security

If you find a security vulnerability related to the Prometheus Operator, please
do not report it by opening a GitHub issue, but instead please send an e-mail to
the maintainers of the project found in the [OWNERS](OWNERS) file.
