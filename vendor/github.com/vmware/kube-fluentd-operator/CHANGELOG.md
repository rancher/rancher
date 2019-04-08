# CHANGELOG

## 1.9.0

## [1.8.0](https://github.com/vmware/kube-fluentd-operator/releases/tag/v1.8.0)

* Fluentd: replace fluent-plugin-amqp plugin with fluent-plugin-amqp2

* Fluentd: update fluentd to 1.2.6

* Fluentd: add plugin `fluent-plugin-vertica` to base image (@rhatlapa)

* Core configmaps per namespace support (#31) (@coufalja)

* Helm: chart updated to 0.3.0 adding support for multiple configmaps per namespace

## [1.7.0](https://github.com/vmware/kube-fluentd-operator/releases/tag/v1.7.0)

* Fluentd: add plugin `extract` to base image - a simple filter plugin to enrich log events based on regex and templates

* Fluentd: add plugin `fluent-plugin-amqp2` to base image (@CoufalJa)

* Fluentd: add plugin `fluent-plugin-grok-parser` to base image (@CoufalJa)

* Core: handle gracefully a missing `kubernetes.labels` field (#23)

* Core: Add `strict true|false` option to the logfmt parser plugin (#27)

* Core: Add `add_labels` to  `@type mounted_file` plug-in (#26)

* Core: Fix `@type mounted_file` for multi-container pods producing log files (#29)

* Helm: `podAnnotations` lets you annotate the daemonset pods (@cw-sakamoto) (#25)

## [1.6.0](https://github.com/vmware/kube-fluentd-operator/releases/tag/v1.6.0)

* Core: plugin macros (#19). This feature lets you reuse output plugin definitions.

* Fluentd: add `fluent-plugin-kafka` to base image (@dimitrovvlado)

* Fluentd: add `fluent-plugin-mail` to base image

* Fluentd: add `fluent-plugin-mongo` to base image

* Fluentd: add `fluent-plugin-scribe` to base image

* Fluentd: add `fluent-plugin-sumologic_output` to base image

## [1.5.0](https://github.com/vmware/kube-fluentd-operator/releases/tag/v1.5.0)

* Core: extreme validation (#15): namespace configs are run in sandbox to catch more errors

* Helm: export a K8S\_NODE\_NAME var to the fluentd container (@databus23)

* Helm: `extraEnv` lets you inject env vars into fluentd

## [1.4.0](https://github.com/vmware/kube-fluentd-operator/releases/tag/v1.4.0)

* Fluentd: add `fluent-plugin-kinesis` to base image (@anton-107)

## [1.3.0](https://github.com/vmware/kube-fluentd-operator/releases/tag/v1.3.0)

* Fluentd: add `fluent-plugin-detect-exceptions` to base image

* Fluentd: add `fluent-plugin-out-http-ext` to base image

* Core: add plugin `truncating_remote_syslog` which truncates the tag to 32 bytes as per RFC 5424

* Core: support transparent multi-line stacktrace collapsing

## [1.2.0](https://github.com/vmware/kube-fluentd-operator/releases/tag/v1.2.0)

* Core: Share log streams between namespaces

* Helm: Mount secrets/configmaps (tls mostly) ala elasticsearch (@sneko)

* Fluentd: Update base-image to fluentd-1.1.3-debian

* Fluentd: Include Splunk plugin into base-image (@mhulscher)

* Fix(Helm): properly set the `resources` field of the reloader container. Setting them had no effect until now (@sneko)

## [1.1.0](https://github.com/vmware/kube-fluentd-operator/releases/tag/v1.1.0)

* Core: ingest log files from a container filesystem

* Core: limit impact of log-router to a set of namespaces using `--namespaces`

* Helm: add new property `kubeletRoot`

* Helm: add new property `namespaces[]`:

## [1.0.1](https://github.com/vmware/kube-fluentd-operator/releases/tag/v1.0.1)

* Fluentd: install plugin `fluent-plugin-concat` in Fluentd

* Core: support for default configmap name using `--default-configmap`

## [1.0.0](https://github.com/vmware/kube-fluentd-operator/releases/tag/v1.0.0)

* Initial version
