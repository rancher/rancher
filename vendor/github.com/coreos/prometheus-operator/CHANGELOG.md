## 0.25.0 / 2018-10-24

* [FEATURE] Allow passing additional alert relabel configs in Prometheus custom resource (#2022)
* [FEATURE] Add ability to mount custom ConfigMaps into Alertmanager and Prometheus (#2028)

## 0.24.0 / 2018-10-11

This release has a breaking changes for `prometheus_operator_.*` metrics.

`prometheus_operator_alertmanager_reconcile_errors_total` and `prometheus_operator_prometheus_reconcile_errors_total`
are now combined and called `prometheus_operator_reconcile_errors_total`.
Instead the metric has a "controller" label which indicates the errors from the Prometheus or Alertmanager controller.

The same happened with `prometheus_operator_alertmanager_spec_replicas` and `prometheus_operator_prometheus_spec_replicas`
which is now called `prometheus_operator_spec_replicas` and also has the "controller" label.

The `prometheus_operator_triggered_total` metric now has a "controller" label as well and finally instruments the
Alertmanager controller.

For a full description see: https://github.com/coreos/prometheus-operator/pull/1984#issue-221139702

In order to support multiple namespaces, the `--namespace` flag changed to `--namespaces`
and accepts and comma-separated list of namespaces as a string.

* [CHANGE] Default to Node Exporter v0.16.0 (#1812)
* [CHANGE] Update to Go 1.11 (#1855)
* [CHANGE] Default to Prometheus v2.4.3 (#1929) (#1983)
* [CHANGE] Default to Thanos v0.1.0 (#1954)
* [CHANGE] Overhaul metrics while adding triggerBy metric for Alertmanager (#1984)
* [CHANGE] Add multi namespace support (#1813)
* [FEATURE] Add SHA field to Prometheus, Alertmanager and Thanos for images (#1847) (#1854)
* [FEATURE] Add configuration for priority class to be assigned to Pods (#1875)
* [FEATURE] Configure sampleLimit per ServiceMonitor (#1895)
* [FEATURE] Add additionalPeers to Alertmanager (#1878)
* [FEATURE] Add podTargetLabels to ServiceMonitors (#1880)
* [FEATURE] Relabel target name for Pods (#1896)
* [FEATURE] Allow configuration of relabel_configs per ServiceMonitor (#1879)
* [FEATURE] Add illegal update reconciliation by deleting StatefulSet (#1931)
* [ENHANCEMENT] Set Thanos cluster and grpc ip from pod.ip (#1836)
* [BUGFIX] Add square brackets around pod IPs for IPv6 support (#1881)
* [BUGFIX] Allow periods in secret name (#1907)
* [BUGFIX] Add BearerToken in generateRemoteReadConfig (#1956)

## 0.23.2 / 2018-08-23

* [BUGFIX] Do not abort kubelet endpoints update due to nodes without IP addresses defined (#1816)

## 0.23.1 / 2018-08-13

* [BUGFIX] Fix high CPU usage of Prometheus Operator when annotating Prometheus resource (#1785)

## 0.23.0 / 2018-08-06

* [CHANGE] Deprecate specification of Prometheus rules via ConfigMaps in favor of `PrometheusRule` CRDs
* [FEATURE] Introduce new flag to control logging format (#1475)
* [FEATURE] Ensure Prometheus Operator container runs as `nobody` user by default (#1393)
* [BUGFIX] Fix reconciliation of Prometheus StatefulSets due to ServiceMonitors and PrometheusRules changes when a single namespace is being watched (#1749)

## 0.22.2 / 2018-07-24

[BUGFIX] Do not migrate rule config map for Prometheus statefulset on rule config map to PrometheusRule migration (#1679)

## 0.22.1 / 2018-07-19

* [ENHANCEMENT] Enable operation when CRDs are created externally (#1640)
* [BUGFIX] Do not watch for new namespaces if a specific namespace has been selected (#1640)

## 0.22.0 / 2018-07-09

* [FEATURE] Allow setting volume name via volumetemplateclaimtemplate in prom and alertmanager (#1538)
* [FEATURE] Allow setting custom tags of container images (#1584) 
* [ENHANCEMENT] Update default Thanos to v0.1.0-rc.2 (#1585)
* [ENHANCEMENT] Split rule config map mounted into Prometheus if it exceeds Kubernetes config map limit (#1562)
* [BUGFIX] Mount Prometheus data volume into Thanos sidecar & pass correct path to Thanos sidecar (#1583)

## 0.21.0 / 2018-06-28

* [CHANGE] Default to Prometheus v2.3.1.
* [CHANGE] Default to Alertmanager v0.15.0.
* [FEATURE] Make remote write queue configurations configurable.
* [FEATURE] Add Thanos integration (experimental).
* [BUGFIX] Fix usage of console templates and libraries.

## 0.20.0 / 2018-06-05

With this release we introduce a new Custom Resource Definition - the
`PrometheusRule` CRD. It addresses the need for rule syntax validation and rule
selection across namespaces. `PrometheusRule` replaces the configuration of
Prometheus rules via K8s ConfigMaps. There are two migration paths:

1. Automated live migration: If the Prometheus Operator finds Kubernetes
   ConfigMaps that match the `RuleSelector` in a `Prometheus` specification, it
   will convert them to matching `PrometheusRule` resources.

2. Manual migration: We provide a basic CLI tool to convert Kubernetes
   ConfigMaps to `PrometheusRule` resources.

```bash
go get -u github.com/coreos/prometheus-operator/cmd/po-rule-migration
po-rule-migration \
--rule-config-map=<path-to-config-map> \
--rule-crds-destination=<path-to-rule-crd-destination>
```

* [FEATURE] Add leveled logging to Prometheus Operator (#1277)
* [FEATURE] Allow additional Alertmanager configuration in Prometheus CRD (#1338)
* [FEATURE] Introduce `PrometheusRule` Custom Resource Definition (#1333)
* [ENHANCEMENT] Allow Prometheus to consider all namespaces to find ServiceMonitors (#1278)
* [BUGFIX] Do not attempt to set default memory request for Prometheus 2.0 (#1275)

## 0.19.0 / 2018-04-25

* [FEATURE] Allow specifying additional Prometheus scrape configs via secret (#1246)
* [FEATURE] Enable Thanos sidecar (#1219)
* [FEATURE] Make AM log level configurable (#1192)
* [ENHANCEMENT] Enable Prometheus to select Service Monitors outside own namespace (#1227)
* [ENHANCEMENT] Enrich Prometheus operator CRD registration error handling (#1208)
* [BUGFIX] Allow up to 10m for Prometheus pod on startup for data recovery (#1232)

## 0.18.1 / 2018-04-09

* [BUGFIX] Fix alertmanager >=0.15.0 cluster gossip communication (#1193)

## 0.18.0 / 2018-03-04

From this release onwards only Kubernetes versions v1.8 and higher are supported. If you have an older version of Kubernetes and the Prometheus Operator running, we recommend upgrading Kubernetes first and then the Prometheus Operator.

While multiple validation issues have been fixed, it will remain a beta feature in this release. If you want to update validations, you need to either apply the CustomResourceDefinitions located in `example/prometheus-operator-crd` or delete all CRDs and restart the Prometheus Operator.

Some changes cause Prometheus and Alertmanager clusters to be redeployed. If you do not have persistent storage backing your data, this means you will loose the amount of data equal to your retention time.

* [CHANGE] Use canonical `/prometheus` and `/alertmanager` as data dirs in containers.
* [FEATURE] Allow configuring Prometheus and Alertmanager servers to listen on loopback interface, allowing proxies to be the ingress point of those Pods.
* [FEATURE] Allow configuring additional containers in Prometheus and Alertmanager Pods.
* [FEATURE] Add ability to whitelist Kubernetes labels to become Prometheus labels.
* [FEATURE] Allow specifying additional secrets for Alertmanager Pods to mount.
* [FEATURE] Allow specifying `bearer_token_file` for Alertmanger configurations of Prometheus objects in order to authenticate with Alertmanager.
* [FEATURE] Allow specifying TLS configuration for Alertmanger configurations of Prometheus objects.
* [FEATURE] Add metrics for reconciliation errors: `prometheus_operator_alertmanager_reconcile_errors_total` and `prometheus_operator_prometheus_reconcile_errors_total`.
* [FEATURE] Support `read_recent` and `required_matchers` fields for remote read configurations.
* [FEATURE] Allow disabling any defaults of `SecurityContext` fields of Pods.
* [BUGFIX] Handle Alertmanager >=v0.15.0 breaking changes correctly.
* [BUGFIX] Fix invalid validations for metric relabeling fields.
* [BUGFIX] Fix validations for `AlertingSpec`.
* [BUGFIX] Fix validations for deprecated storage fields.
* [BUGFIX] Fix remote read and write basic auth support.
* [BUGFIX] Fix properly propagate errors of Prometheus config reloader.

## 0.17.0 / 2018-02-15

This release adds validations as a beta feature. It will only be installed on new clusters, existing CRD definitions will not be updated, this will be done in a future release. Please try out this feature and give us feedback!

* [CHANGE] Default Prometheus version v2.2.0-rc.0.
* [CHANGE] Default Alertmanager version v0.14.0.
* [FEATURE] Generate and add CRD validations.
* [FEATURE] Add ability to set `serviceAccountName` for Alertmanager Pods.
* [FEATURE] Add ability to specify custom `securityContext` for Alertmanager Pods.
* [ENHANCEMENT] Default to non-root security context for Alertmanager Pods.

## 0.16.1 / 2018-01-16

* [CHANGE] Default to Alertmanager v0.13.0.
* [BUGFIX] Alertmanager flags must be double dashed starting v0.13.0.

## 0.16.0 / 2018-01-11

* [FEATURE] Add support for specifying remote storage configurations.
* [FEATURE] Add ability to specify log level.
* [FEATURE] Add support for dropping metrics at scrape time.
* [ENHANCEMENT] Ensure that resource limit can't make Pods unschedulable.
* [ENHANCEMENT] Allow configuring emtpyDir volumes
* [BUGFIX] Use `--storage.tsdb.no-lockfile` for Prometheus 2.0.
* [BUGFIX] Fix Alertmanager default storage.path.

## 0.15.0 / 2017-11-22

* [CHANGE] Default Prometheus version v2.0.0
* [BUGFIX] Generate ExternalLabels deterministically
* [BUGFIX] Fix incorrect mount path of Alertmanager data volume
* [EXPERIMENTAL] Add ability to specify CRD Kind name

## 0.14.1 / 2017-11-01

* [BUGFIX] Ignore illegal change of PodManagementPolicy to StatefulSet.

## 0.14.0 / 2017-10-19

* [CHANGE] Default Prometheus version v2.0.0-rc.1.
* [CHANGE] Default Alertmanager version v0.9.1.
* [BUGFIX] Set StatefulSet replicas to 0 if 0 is specified in Alertmanager/Prometheus object.
* [BUGFIX] Glob for all files in a ConfigMap as rule files.
* [FEATURE] Add ability to run Prometheus Operator for a single namespace.
* [FEATURE] Add ability to specify CRD api group.
* [FEATURE] Use readiness and health endpoints of Prometheus 1.8+.
* [ENHANCEMENT] Add OwnerReferences to managed objects.
* [ENHANCEMENT] Use parallel pod creation strategy for Prometheus StatefulSets.

## 0.13.0 / 2017-09-21

After a long period of not having broken any functionality in the Prometheus Operator, we have decided to promote the status of this project to beta.

Compatibility guarantees and migration strategies continue to be the same as for the `v0.12.0` release.

* [CHANGE] Remove analytics collection.
* [BUGFIX] Fix memory leak in kubelet endpoints sync.
* [FEATURE] Allow setting global default `scrape_interval`.
* [FEATURE] Allow setting Pod objectmeta to Prometheus and Alertmanger objects.
* [FEATURE] Allow setting tolerations and affinity for Prometheus and Alertmanager objects.

## 0.12.0 / 2017-08-24

Starting with this release only Kubernetes `v1.7.x` and up is supported as CustomResourceDefinitions are a requirement for the Prometheus Operator and are only available from those versions and up.

Additionally all objects have been promoted from `v1alpha1` to `v1`. On start up of this version of the Prometheus Operator the previously used `ThirdPartyResource`s and the associated `v1alpha1` objects will be automatically migrated to their `v1` equivalent `CustomResourceDefinition`.

* [CHANGE] All manifests created and used by the Prometheus Operator have been promoted from `v1alpha1` to `v1`.
* [CHANGE] Use Kubernetes `CustomResourceDefinition`s instead of `ThirdPartyResource`s.
* [FEATURE] Add ability to set scrape timeout to `ServiceMonitor`.
* [ENHANCEMENT] Use `StatefulSet` rolling deployments.
* [ENHANCEMENT] Properly set `SecurityContext` for Prometheus 2.0 deployments.
* [ENHANCEMENT] Enable web lifecycle APIs for Prometheus 2.0 deployments.

## 0.11.2 / 2017-09-21

* [BUGFIX] Fix memory leak in kubelet endpoints sync.

## 0.11.1 / 2017-07-28

* [ENHANCEMENT] Add profiling endpoints.
* [BUGFIX] Adapt Alertmanager storage usage to not use deprecated storage definition.

## 0.11.0 / 2017-07-20

Warning: This release deprecates the previously used storage definition in favor of upstream PersistentVolumeClaim templates. While this should not have an immediate effect on a running cluster, Prometheus object definitions that have storage configured need to be adapted. The previously existing fields are still there, but have no effect anymore.

* [FEATURE] Add Prometheus 2.0 alpha3 support.
* [FEATURE] Use PVC templates instead of custom storage definition.
* [FEATURE] Add cAdvisor port to kubelet sync.
* [FEATURE] Allow default base images to be configurable.
* [FEATURE] Configure Prometheus to only use necessary namespaces.
* [ENHANCEMENT] Improve rollout detection for Alertmanager clusters.
* [BUGFIX] Fix targetPort relabeling.

## 0.10.2 / 2017-06-21

* [BUGFIX] Use computed route prefix instead of directly from manifest.

## 0.10.1 / 2017-06-13

Attention: if the basic auth feature was previously used, the `key` and `name`
fields need to be switched. This was not intentional, and the bug is not fixed,
but causes this change.

* [CHANGE] Prometheus default version v1.7.1.
* [CHANGE] Alertmanager default version v0.7.1.
* [BUGFIX] Fix basic auth secret key selector `key` and `name` switched.
* [BUGFIX] Fix route prefix flag not always being set for Prometheus.
* [BUGFIX] Fix nil panic on replica metrics.
* [FEATURE] Add ability to specify Alertmanager path prefix for Prometheus.

## 0.10.0 / 2017-06-09

* [CHANGE] Prometheus route prefix defaults to root.
* [CHANGE] Default to Prometheus v1.7.0.
* [CHANGE] Default to Alertmanager v0.7.0.
* [FEATURE] Add route prefix support to Alertmanager resource.
* [FEATURE] Add metrics on expected replicas.
* [FEATURE] Support for runing Alertmanager v0.7.0.
* [BUGFIX] Fix sensitive rollout triggering.

## 0.9.1 / 2017-05-18

* [FEATURE] Add experimental Prometheus 2.0 support.
* [FEATURE] Add support for setting Prometheus external labels.
* [BUGFIX] Fix non-deterministic config generation.

## 0.9.0 / 2017-05-09

* [CHANGE] The `kubelet-object` flag has been renamed to `kubelet-service`.
* [CHANGE] Remove automatic relabelling of Pod and Service labels onto targets.
* [CHANGE] Remove "non-namespaced" alpha annotation in favor of `honor_labels`.
* [FEATURE] Add ability make use of the Prometheus `honor_labels` configuration option.
* [FEATURE] Add ability to specify image pull secrets for Prometheus and Alertmanager pods.
* [FEATURE] Add basic auth configuration option through ServiceMonitor.
* [ENHANCEMENT] Add liveness and readiness probes to Prometheus and Alertmanger pods.
* [ENHANCEMENT] Add default resource requests for Alertmanager pods.
* [ENHANCEMENT] Fallback to ExternalIPs when InternalIPs are not available in kubelet sync.
* [ENHANCEMENT] Improved change detection to trigger Prometheus rollout.
* [ENHANCEMENT] Do not delete unmanaged Prometheus configuration Secret.

## 0.8.2 / 2017-04-20

* [ENHANCEMENT] Use new Prometheus 1.6 storage flags and make it default.

## 0.8.1 / 2017-04-13

* [ENHANCEMENT] Include kubelet insecure port in kubelet Enpdoints object.

## 0.8.0 / 2017-04-07

* [FEATURE] Add ability to mount custom secrets into Prometheus Pods. Note that
  secrets cannot be modified after creation, if the list if modified after
  creation it will not effect the Prometheus Pods.
* [FEATURE] Attach pod and service name as labels to Pod targets.

## 0.7.0 / 2017-03-17

This release introduces breaking changes to the generated StatefulSet's
PodTemplate, which cannot be modified for StatefulSets. The Prometheus and
Alertmanager objects have to be deleted and recreated for the StatefulSets to
be created properly.

* [CHANGE] Use Secrets instead of ConfigMaps for configurations.
* [FEATURE] Allow ConfigMaps containing rules to be selected via label selector.
* [FEATURE] `nodeSelector` added to the Alertmanager kind.
* [ENHANCEMENT] Use Prometheus v2 chunk encoding by default.
* [BUGFIX] Fix Alertmanager cluster mesh initialization.

## 0.6.0 / 2017-02-28

* [FEATURE] Allow not tagging targets with the `namespace` label.
* [FEATURE] Allow specifying `ServiceAccountName` to be used by Prometheus pods.
* [ENHANCEMENT] Label governing services to uniquely identify them.
* [ENHANCEMENT] Reconcile Serive and Endpoints objects.
* [ENHANCEMENT] General stability improvements.
* [BUGFIX] Hostname cannot be fqdn when syncing kubelets into Endpoints object.

## 0.5.1 / 2017-02-17

* [BUGFIX] Use correct governing `Service` for Prometheus `StatefulSet`.

## 0.5.0 / 2017-02-15

* [FEATURE] Allow synchronizing kubelets into an `Endpoints` object.
* [FEATURE] Allow specifying custom configmap-reload image

## 0.4.0 / 2017-02-02

* [CHANGE] Split endpoint and job in separate labels instead of a single
  concatenated one.
* [BUGFIX] Properly exit on errors communicating with the apiserver.

## 0.3.0 / 2017-01-31

This release introduces breaking changes to the underlying naming schemes. It
is recommended to destroy existing Prometheus and Alertmanager objects and
recreate them with new namings.

With this release support for `v1.4.x` clusters is dropped. Changes will not be
backported to the `0.1.x` release series anymore.

* [CHANGE] Prefixed StatefulSet namings based on managing resource
* [FEATURE] Pass labels and annotations through to StatefulSets
* [FEATURE] Add tls config to use for Prometheus target scraping
* [FEATURE] Add configurable `routePrefix` for Prometheus
* [FEATURE] Add node selector to Prometheus TPR
* [ENHANCEMENT] Stability improvements

## 0.2.3 / 2017-01-05

* [BUGFIX] Fix config reloading when using external url.

## 0.1.3 / 2017-01-05

The `0.1.x` releases are backport releases with Kubernetes `v1.4.x` compatibility.

* [BUGFIX] Fix config reloading when using external url.

## 0.2.2 / 2017-01-03

* [FEATURE] Add ability to set the external url the Prometheus/Alertmanager instances will be available under.

## 0.1.2 / 2017-01-03

The `0.1.x` releases are backport releases with Kubernetes `v1.4.x` compatibility.

* [FEATURE] Add ability to set the external url the Prometheus/Alertmanager instances will be available under.

## 0.2.1 / 2016-12-23

* [BUGFIX] Fix `subPath` behavior when not using storage provisioning

## 0.2.0 / 2016-12-20

This release requires a Kubernetes cluster >=1.5.0. See the readme for
instructions on how to upgrade if you are currently running on a lower version
with the operator.

* [CHANGE] Use StatefulSet instead of PetSet
* [BUGFIX] Fix Prometheus config generation for labels containing "-"
