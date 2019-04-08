# kube-fluentd-operator  [![Build Status](https://travis-ci.org/vmware/kube-fluentd-operator.svg?branch=master)](https://travis-ci.org/vmware/kube-fluentd-operator)

[![Go Report Card](https://goreportcard.com/badge/github.com/vmware/kube-fluentd-operator)](https://goreportcard.com/report/github.com/vmware/kube-fluentd-operator)

## Overview

TL;DR: a sane, no-brainer K8S+Helm distribution of Fluentd with batteries included, config validation, no needs to restart, with sensible defaults and best practices built-in. Use Kubernetes labels to filter/route logs!

*kube-fluentd-operator* configures Fluentd in a Kubernetes environment. It compiles a Fluentd configuration from configmaps (one per namespace) - similar to how an Ingress controller would compile nginx configuration from several Ingress resources. This way only one instance of Fluentd can handle all log shipping while the cluster admin need NOT coordinate with namespace admins.

Cluster administrators set up Fluentd once only and namespace owners can configure log routing as they wish. *kube-fluentd-operator* will re-configure Fluentd accordingly and make sure logs originating from a namespace will not be accessible by other tenants/namespaces.

*kube-fluentd-operator* also extends the Fluentd configuration language making it possible to refer to pods based on their labels and the container. This enables for very fined-grained targeting of log streams for the purpose of pre-processing before shipping.

Finally, it is possible to ingest logs from a file on the container filesystem. While this is not recommended, there are still legacy or misconfigured apps that insist on logging to the local filesystem.

## Try it out

The easiest way to get started is using the Helm chart. Official images are not published yet, so you need to pass the image.repository and image.tag manually:

```bash
git clone git@github.com:vmware/kube-fluentd-operator.git
helm install --name kfo ./kube-fluentd-operator/log-router \
  --set rbac.create=true \
  --set image.tag=v1.8.0 \
  --set image.repository=jvassev/kube-fluentd-operator
```

Alternatively, deploy the Helm chart from a Github release:

```bash
CHART_URL='https://github.com/vmware/kube-fluentd-operator/releases/download/v1.8.0/log-router-0.3.0.tgz'

helm install --name kfo ${CHART_URL} \
  --set rbac.create=true \
  --set image.tag=v1.8.0 \
  --set image.repository=jvassev/kube-fluentd-operator
```

Then create a namespace `demo` and a configmap describing where all logs from `demo` should go to. The configmap must contain an entry called "fluent.conf". Finally, point the kube-fluentd-operator to this configmap using annotations.

```bash
kubectl create ns demo

cat > fluent.conf << EOF
<match **>
  @type null
</match>
EOF

# Create the configmap with a single entry "fluent.conf"
kubectl create configmap fluentd-config --namespace demo --from-file=fluent.conf=fluent.conf


# The following step is optional: the fluentd-config is the default configmap name.
# kubectl annotate demo logging.csp.vmware.com/fluentd-configmap=fluentd-config

```

In a minute, this configuration would be translated to something like this:

```xml
<match demo.**>
  @type null
</match>
```

Even though the tag `**` was used in the `<match>` directive, the kube-fluentd-operator correctly expands this to `demo.**`. Indeed, if another tag which does not start with `demo.` was used, it would have failed validation. Namespace admins can safely assume that they has a dedicated Fluentd for themselves.

All configuration errors are stored in the annotation `logging.csp.vmware.com/fluentd-status`. Try replacing `**` with an invalid tag like 'hello-world'. After a minute, verify that the error message looks like this:

```bash
# extract just the value of logging.csp.vmware.com/fluentd-status
kubectl get ns demo -o jsonpath='{.metadata.annotations.logging\.csp\.vmware\.com/fluentd-status}'
bad tag for <match>: hello-world. Tag must start with **, $thins or demo
```

When the configuration is made valid again the `fluentd-status` is set to "".

To see kube-fluentd-operator in action you need a cloud log collector like logz.io, loggly, papertrail or ELK accessible from the K8S cluster. A simple loggly configuration looks like this (replace TOKEN with your customer token):

```xml
<match **>
   @type loggly
   loggly_url https://logs-01.loggly.com/inputs/TOKEN/tag/fluentd
</match>
```

## Build

Get the code using `go get`:

```bash
go get -u github.com/vmware/kube-fluentd-operator/config-reloader
cd $GOPATH/src/github.com/vmware/kube-fluentd-operator

# build a base-image
cd base-image && make

# build helm chart
cd log-router && make

# build the daemon
cd config-reloader
# get vendor/ deps, you need to have "dep" tool installed
make dep
make install

# run with mock data (loaded from the examples/ folder)
make run-local-fs

# inspect what is generated from the above command
ls -l tmp/
```

### Project structure

* `log-router`: Builds the Helm chart
* `base-image`: Builds a Fluentd 1.2.x image with a curated list of plugins
* `config-reloader`: Builds the daemon that generates fluentd configuration files

### Config-reloader

This is where interesting work happens. The [dependency graph](config-reloader/godepgraph.png) shows the high-level package interaction and general dataflow.

* `config`: handles startup configuration, reading and validation
* `datasource`: fetches Pods, Namespaces, ConfigMaps from Kubernetes
* `fluentd`: parses Fluentd config files into an object graph
* `processors`: walks this object graph doing validations and modifications. All features are implemented as chained `Processor` subtypes
* `generator`: serializes the processed object graph to the filesystem for Fluentd to read
* `controller`: orchestrates the high-level `datasource` -> `processor` -> `generator` pipeline.

### How does it work

It works be rewriting the user-provided configuration. This is possible because *kube-fluentd-operator* knows about the kubernetes cluster, the current namespace and
also has some sensible defaults built in. To get a quick idea what happens behind the scenes consider this configuration deployed in a namespace called `monitoring`:

```xml
<filter $labels(server=apache)>
  @type parse
  format apache2
</filter>

<filter $labels(app=django)>
  @type detect_exceptions
  language python
</filter>

<match **>
  @type es
</match>
```

It gets processed into the following configuration which is then fed to Fluentd:

```xml
<filter kube.monitoring.*.*>
  @type record_transformer
  enable_ruby true

  <record>
    kubernetes_pod_label_values ${record["kubernetes"]["labels"]["app"]&.gsub(/[.-]/, '_') || '_'}.${record["kubernetes"]["labels"]["server"]&.gsub(/[.-]/, '_') || '_'}
  </record>
</filter>

<match kube.monitoring.*.*>
  @type rewrite_tag_filter

  <rule>
    key kubernetes_pod_label_values
    pattern ^(.+)$
    tag ${tag}._labels.$1
  </rule>
</match>

<filter kube.monitoring.*.*.**>
  @type record_transformer
  remove_keys kubernetes_pod_label_values
</filter>

<filter kube.monitoring.*.*._labels.*.apache _proc.kube.monitoring.*.*._labels.*.apache>
  @type parse
  format apache2
</filter>

<match kube.monitoring.*.*._labels.django.*>
  @type rewrite_tag_filter

  <rule>
    invert true
    key _dummy
    pattern /ZZ/
    tag 3bfd045d94ce15036a8e3ff77fcb470e0e02ebee._proc.${tag}
  </rule>
</match>

<match 3bfd045d94ce15036a8e3ff77fcb470e0e02ebee._proc.kube.monitoring.*.*._labels.django.*>
  @type detect_exceptions
  remove_tag_prefix 3bfd045d94ce15036a8e3ff77fcb470e0e02ebee
  stream container_info
</match>

<match kube.monitoring.*.*._labels.*.* _proc.kube.monitoring.*.*._labels.*.*>
  @type es
</match>
```

## Configuration

### Basic usage

To give the illusion that every namespace runs a dedicated Fluentd the user-provided configuration is post-processed. In general, expressions starting with `$` are macros that are expanded. These two directives are equivalent: `<match **>`, `<match $thisns>`. Almost always, using the `**` is the preferred way to match logs: this way you can reuse the same configuration for multiple namespaces.

### A note on the `kube-system` namespace

The `kube-system` is treated differently. Its configuration is not processed further as it is assumed only the cluster admin can manipulate resources in this namespace. If you don't plan to use any of the advanced features described bellow, you can just route all logs from all namespaces using this snippet at the `kube-system` level:

```xml
<match **>
 @type ...
 # destination configuration omitted
</match>
```

`**` in this context is not processed and it means *literally* everything.

Fluentd assumes it is running in a distro with systemd and generates logs with these Fluentd tags:

* `systemd.{unit}`: the journal of a systemd unit, for example `systemd.docker.service`
* `docker`: all docker logs, not containers. If systemd is used, the docker logs are in `systemd.docker.service`
* `k8s.{component}`: logs from a K8S component, for example `k8s.kube-apiserver`
* `kube.{namespace}.{pod_name}.{container_name}`: a log originating from (namespace, pod, container)

As `kube-system` is processed first, a match-all directive would consume all logs and any other namespace configuration will become irrelevant (unless `<copy>` is used).
A recommended configuration for the `kube-system` namespace is this one - it captures all but the user namespaces' logs:

```xml
<match systemd.** kube.kube-system.** k8s.** docker>
  # all k8s-internal and OS-level logs

  # destination config omitted...
</match>
```

Note the `<match systemd.**` syntax. A single `*` would not work as the tag is the full name - including the unit type, for example *systemd.nginx.service*

### Using the $labels macro

A very useful feature is the `<filter>` and the `$labels` macro to define parsing at the namespace level. For example, the config-reloader container uses the `logfmt` format. This makes it easy to use structured logging and ingest json data into a remote log ingestion service.

```xml
<filter $labels(app=log-router, _container=reloader)>
  @type parse
  format logfmt
  reserve_data true
</filter>

<match **>
  @type loggly
  # destination config omitted
</match>
```

The above config will pipe all logs from the pods labelled with `app=log-router` through a [logfmt](https://github.com/vmware/kube-fluentd-operator/blob/master/base-image/plugins/parser_logfmt.rb) parser before sending them to loggly. Again, this configuration is valid in any namespace. If the namespace doesn't contain any `log-router` components then the `<filter>` directive is never activated. The `_container` is sort of a "meta" label and it allows for targeting the log stream of a specific container in a multi-container pod.

All plugins that change the fluentd tag are disabled for security reasons. Otherwise a rogue configuration may divert other namespace's logs to itself by prepending its name to the tag.

### Ingest logs from a file in the container

The only allowed `<source>` directive is of type `mounted-file`. It is used to ingest a log file from a container on an `emptyDir`-mounted volume:

```xml
<source>
  @type mounted-file
  path /var/log/welcome.log
  labels app=grafana, _container=test-container
  <parse>
    @type none
  </parse>
</source>
```

The `labels` parameter is similar to the `$labels` macro and helps the daemon locate all pods that might log to the given file path. The `<parse>` directive is optional and if omitted the default `@type none` will be used. If you know the format of the log file you can explicitly specify it, for example `@type apache2` or `@type json`.

The above configuration would translate at runtime to something similar to this:

```xml
<source>
  @type tail
  path /var/lib/kubelet/pods/723dd34a-4ac0-11e8-8a81-0a930dd884b0/volumes/kubernetes.io~empty-dir/logs/welcome.log
  pos_file /var/log/kfotail-7020a0b821b0d230d89283ba47d9088d9b58f97d.pos
  read_from_head true
  tag kube.kfo-test.welcome-logger.test-container

  <parse>
    @type none
  </parse>
</source>
```

### Dealing with multi-line exception stacktraces (since v1.3.0)

Most log streams are line-oriented. However, stacktraces always span multiple lines. *kube-fluentd-operator* integrates stacktrace processing using the [fluent-plugin-detect-exceptions](https://github.com/GoogleCloudPlatform/fluent-plugin-detect-exceptions). If a Java-based pod produces stacktraces in the logs, then the stacktraces can be collapsed in a single log event like this:

```xml
<filter $labels(app=jpetstore)>
  @type detect_exceptions
  # you can skip language in which case all possible languages will be tried: go, java, python, ruby, etc...
  language java
</filter>

# The rest of the configuration stays the same even though quite a lot of tag rewriting takes place

<match **>
 @type es
</match>
```

Notice how `filter` is used instead of `match` as described in [fluent-plugin-detect-exceptions](https://github.com/GoogleCloudPlatform/fluent-plugin-detect-exceptions). Internally, this filter is translated into several `match` directives so that the end user doesn't need to bother with rewriting the Fluentd tag.

Also, users don't need to bother with setting the correct `stream` parameter. *kube-fluentd-operator* generates one internally based on the container id and the stream.

### Reusing output plugin definitions (since v1.6.0)

Sometimes you only have a few valid options for log sinks: a dedicated S3 bucket, the ELK stack you manage, etc. The only flexibility you're after is letting namespace owners filter and parse their logs. In such cases you can abstract over an output plugin configuration - basically reducing it to a simple name which can be referenced from any namespace. For example, let's assume you have an S3 bucket for a "test" environement and you use loggly for a "staging" environment. The first thing you do is define these two output at the `kube-system` level:

```xml
kube-system.conf:
<plugin test>
  @type s3
  aws_key_id  YOUR_AWS_KEY_ID
  aws_sec_key YOUR_AWS_SECRET_KEY
  s3_bucket   YOUR_S3_BUCKET_NAME
  s3_region   AWS_REGION
</plugin>

<plugin staging>
  @type loggly
  loggly_url https://logs-01.loggly.com/inputs/TOKEN/tag/fluentd
</plugin>
```

A namespace can refer to the `staging` and `test` plugins oblivious to the fact where exactly the logs end up:

```xml
acme-test.conf
<match **>
  @type test
</match>


acme-staging.conf
<match **>
  @type staging
</match>
```

kube-fluentd-operator will insert the content of the `plugin` directive in the `match` directive. From then on, regular validation and postprocessing takes place.

### Sharing logs between namespaces

By default, you can consume logs only from your namespaces. Often it is useful for multiple namespaces (tenants) to get access to the logs streams of a shared resource (pod, namespace). *kube-fluentd-operator* makes it possible using two constructs: the source namespace expresses its intent to share logs with a destination namespace and the destination namespace expresses its desire to consume logs from a source. As a result logs are streamed only when both sides agree.

A source namespace can share with another namespace using the `@type share` macro:

producer namespace configuration:

```xml
<match $labels(msg=nginx-ingress)>
  @type copy
  <store>
    @type share
    # share all logs matching the labels with the namespace "consumer"
    with_namespace consumer
  </store>
</match>
```

consumer namespace configuration:

```xml
# use $from(producer) to get all shared logs from a namespace called "producer"
<label @$from(producer)>
  <match **>
    # process all shared logs here as usual
  </match>
</match>
```

The consuming namespace can use the usual syntax inside the `<label @$from...>` directive. The fluentd tag is being rewritten as if the logs originated from the same namespace.

The producing namespace need to wrap `@type share` within a `<store>` directive. This is done on purpose as it is very easy to just redirect the logs to the destination namespace and lose them. The `@type copy` clones the whole stream.

### Log metadata

Often you run mulitple Kubernetes clusters but you need to aggregate all logs to a single destination. To distinguish between different sources, `kube-fluentd-operator` can attach arbitrary metadata to every log event.
The metadata is nested under a key chosen with `--meta-key`. Using the helm chart, metadata can be enabled like this:

```bash
helm instal ... \
  --set meta.key=metadata \
  --set meta.values.region=us-east-1 \
  --set meta.values.env=staging \
  --set meta.values.cluster=legacy
```

Every log event, be it from a pod, mounted-file or a systemd unit, will now carry this metadata:

```json
{
  "metadata": {
    "region": "us-east-1",
    "env": "staging",
    "cluster": "legacy",
  }
}
```

All logs originating from a file look exactly as all other Kubernetes logs. However, their `stream` field is not set to `stdout` but to the path to the source file:

```json
{
    "message": "Some message from the welcome-logger pod",
    "stream": "/var/log/welcome.log",
    "kubernetes": {
        "container_name": "test-container",
        "host": "ip-11-11-11-11.us-east-2.compute.internal",
        "namespace_name": "kfo-test",
        "pod_id": "723dd34a-4ac0-11e8-8a81-0a930dd884b0",
        "pod_name": "welcome-logger",
        "labels": {
            "msg": "welcome",
            "test-case": "b"
        },
        "namespace_labels": {}
    },
    "metadata": {
        "region": "us-east-2",
        "cluster": "legacy",
        "env": "staging"
    }
}
```

## Available plugins in latest release (1.8.0)

`kube-fluentd-operator` aims to be easy to use and flexible. It also favors sending logs to multiple destinations using `<copy>` and as such comes with many plugins pre-installed:

* fluent-config-regexp-type (1.0.0)
* fluent-mixin-config-placeholders (0.4.0)
* fluent-plugin-amqp (0.12.0)
* fluent-plugin-concat (2.3.0)
* fluent-plugin-detect-exceptions (0.0.11)
* fluent-plugin-elasticsearch (3.0.1)
* fluent-plugin-google-cloud (0.7.3)
* fluent-plugin-grok-parser (2.4.0)
* fluent-plugin-kafka (0.8.3)
* fluent-plugin-kinesis (2.1.1)
* fluent-plugin-kubernetes (0.3.1)
* fluent-plugin-kubernetes_metadata_filter (2.1.6)
* fluent-plugin-logentries (0.2.10)
* fluent-plugin-loggly (0.0.9)
* fluent-plugin-logzio (0.0.19)
* fluent-plugin-mail (0.3.0)
* fluent-plugin-mongo (1.2.1)
* fluent-plugin-out-http-ext (0.1.10)
* fluent-plugin-papertrail (0.2.6)
* fluent-plugin-parser (0.6.1)
* fluent-plugin-prometheus (1.3.0)
* fluent-plugin-record-modifier (1.1.0)
* fluent-plugin-record-reformer (0.9.1)
* fluent-plugin-redis (0.3.3)
* fluent-plugin-remote_syslog (1.0.0)
* fluent-plugin-rewrite-tag-filter (2.1.1)
* fluent-plugin-route (1.0.0)
* fluent-plugin-s3 (1.1.7)
* fluent-plugin-scribe (1.0.0)
* fluent-plugin-secure-forward (0.4.5)
* fluent-plugin-splunkhec (1.7)
* fluent-plugin-sumologic_output (1.3.2)
* fluent-plugin-systemd (1.0.1)
* fluent-plugin-vertica (0.0.2)
* fluent-plugin-verticajson (0.0.5)
* fluentd (1.2.6, 1.2.5, 0.12.43, 0.10.62)


When customizing the image be careful not to uninstall plugins that are used internally to implement macros.

If you need other destination plugins you are welcome to contribute a patch or just create an issue.

## Synopsis

The config-reloader binary is the one that listens to changes in K8S and generates Fluentd files. It runs as a daemonset and is not intended to interact with directly. The synopsis is useful when trying to understand the Helm chart or just hacking.

```txt
usage: config-reloader [<flags>]

Regenerates Fluentd configs based Kubernetes namespace annotations against templates, reloading
Fluentd if necessary

Flags:
  --help                        Show context-sensitive help (also try --help-long and
                                --help-man).
  --version                     Show application version.
  --master=""                   The Kubernetes API server to connect to (default: auto-detect)
  --kubeconfig=""               Retrieve target cluster configuration from a Kubernetes
                                configuration file (default: auto-detect)
  --datasource=default          Datasource to use
  --fs-dir=FS-DIR               If datasource=fs is used, configure the dir hosting the files
  --interval=60                 Run every x seconds
  --allow-file                  Allow @type file for namespace configuration
  --id="default"                The id of this deployment. It is used internally so that two
                                deployments don't overwrite each other's data
  --fluentd-rpc-port=24444      RPC port of Fluentd
  --log-level="info"            Control verbosity of log
  --annotation="logging.csp.vmware.com/fluentd-configmap"
                                Which annotation on the namespace stores the configmap name?
  --default-configmap="fluentd-config"
                                Read the configmap by this name if namespace is not annotated.
                                Use empty string to suppress the default.
  --status-annotation="logging.csp.vmware.com/fluentd-status"
                                Store configuration errors in this annotation, leave empty to
                                turn off
  --kubelet-root="/var/lib/kubelet/"
                                Kubelet root dir, configured using --root-dir on the kubelet
                                service
  --namespaces=NAMESPACES ...   List of namespaces to process. If empty, processes all namespaces
  --templates-dir="/templates"  Where to find templates
  --output-dir="/fluentd/etc"   Where to output config files
  --meta-key=META-KEY           Attach metadat under this key
  --meta-values=META-VALUES     Metadata in the k=v,k2=v2 format
  --fluentd-binary=FLUENTD-BINARY
                                Path to fluentd binary used to validate configuration
  --prometheus-enabled          Prometheus metrics enabled (default: false)

```

## Helm chart

| Parameter                                | Description                         | Default                                           |
|------------------------------------------|-------------------------------------|---------------------------------------------------|
| `rbac.create`                            | Create a serviceaccount+role, use if K8s is using RBAC        | `false`                  |
| `serviceAccountName`                     | Reuse an existing service account                | `""`                                             |
| `image.repositiry`                       | Repository                 | `jvassev/kube-fluentd-operator`                              |
| `image.tag`                              | Image tag                | `latest`                          |
| `image.pullPolicy`                       | Pull policy                 | `Always`                             |
| `image.pullSecret`                       | Optional pull secret name                 | `""`                                |
| `logLevel`                               | Default log level                 | `info`                               |
| `kubeletRoot`                            | The home dir of the kubelet, usually set using `--root-dir` on the kubelet           | `/var/lib/kubelet`                               |
| `namespaces`                             | List of namespaces to operate on. Empty means all namespaces                 | `[]`                               |
| `interval`                               | How often to check for config changes (seconds)                 | `45`          |
| `meta.key`                               | The metadata key (optional)                 | `""`                                |
| `meta.values`                            | Metadata to use for the key   | `{}`
| `extraVolumes`                           | Extra volumes                               |                                                            |
| `fluentd.extraVolumeMounts`              | Mount extra volumes for the fluentd container, required to mount ssl certificates when elasticsearch has tls enabled |          |
| `fluentd.resources`                      | Resource definitions for the fluentd container                 | `{}`|
| `fluentd.extraEnv`                       | Extra env vars to pass to the fluentd container           | `{}`                     |
| `reloader.extraVolumeMounts`             | Mount extra volumes for the reloader container |          |
| `reloader.resources`                     | Resource definitions for the reloader container              | `{}`                     |
| `reloader.extraEnv`                      | Extra env vars to pass to the reloader container           | `{}`                     |
| `tolerations`                            | Pod tolerations             | `[]`                     |
| `updateStrategy`                         | UpdateStrategy for the daemonset. Leave empty to get the K8S' default (probably the safest choice)            | `{}`                     |
| `podAnnotations`                         | Pod annotations for the daemonset  |                    |

## Cookbook

### I want to use one destination for everything

Simple, define configuration only for the kube-system namespace:

```bash
kube-system.conf:
<match **>
  # configure destination here
</match>
```

### I dont't care for systemd and docker logs

Simple, exclude them at the kube-system level:

```bash
kube-system.conf:
<match systemd.** docker>
  @type null
</match>

<match **>
  # all but systemd.** is still around
  # configure destination
</match>
```

### I want to use one destination but also want to just exclude a few pods

It is not possible to handle this globally. Instead, provide this config for the noisy namespace and configure other namespaces at the cost of some code duplication:

```xml
noisy-namespace.conf:
<match $labels(app=verbose-logger)>
  @type null
</match>

# all other logs are captured here
<match **>
  @type ...
</match>
```

On the bright side, the configuration of `noisy-namespace` contains nothing specific to noisy-namespace and the same content can be used for all namespaces whose logs we need collected.

### I am getting errors "namespaces is forbidden: ... cannot list namespaces at the cluster scope"

Your cluster is running under RBAC. You need to enable a serviceaccount for the log-router pods. It's easy when using the Helm chart:

```bash
helm install ./log-router --set rbac.create=true ...
```

### I have a legacy container that logs to /var/log/httpd/access.log

First you need version 1.1.0 or later. At the namespace level you need to add a `source` directive of type `mounted-file`:

```xml
<source>
  @type mounted-file
  path /var/log/httpd/access.log
  labels app=apache2
  <parse>
    @type apache2
  </parse>
</source>

<match **>
  # destination config omitted
</match>
```

The type `mounted-file` is again a macro that is expanded to a `tail` plugin. The `<parse>` directive is optional and if not set a `@type none` will be used instead.

In order for this to work the pod must define a mount of type `emptyDir` at `/var/log/httpd` or any of it parent folders. For example, this pod definition is part of the test suite (it logs to /var/log/hello.log):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: hello-logger
  namespace: kfo-test
  labels:
    msg: hello
spec:
  containers:
  - image: ubuntu
    name: greeter
    command:
    - bash
    - -c
    - while true; do echo `date -R` [INFO] "Random hello number $((var++)) to file"; sleep 2; [[ $(($var % 100)) == 0 ]] && :> /var/log/hello.log ;done > /var/log/hello.log
    volumeMounts:
    - mountPath: /var/log
      name: logs
  volumes:
  - name: logs
    emptyDir: {}
```
To get the hello.log ingested by Fluetd you need at least this in the configuration for `kfo-test` namespace:

```xml
<source>
  @type mounted-file
  # need to specify the path on the container filesystem
  path /var/log/hello.log

  # only look at pods labeled this way
  labels msg=hello
  <parse>
    @type none
  </parse>
</source>

<match $labels(msg=hello)>
  # store the hello.log somewhere
  @type ...
</match>
```

### I want to push logs from namespace `demo` to logz.io

```xml
demo.conf:
<match **>
  @type logzio_buffered
  endpoint_url https://listener.logz.io:8071?token=TOKEN&type=log-router
  output_include_time true
  output_include_tags true
  buffer_type    file
  buffer_path    /var/log/kube-system-logz.buf
  flush_interval 10s
  buffer_chunk_limit 1m
</match>
```

For details you should consult the plugin documentation.

### I want to push logs to a remote syslog server

The built-in `remote_syslog` plugin cannot be used as the fluentd tag may be longer than 32 bytes. For this reason there is a `truncating_remote_syslog` plugin that shortens the tag to the allowed limit. If you are currently using the `remote_syslog` output plugin you only need to change a single line:

```xml
<match **>
  # instead of "remote_syslog"
  @type truncating_remote_syslog

  # the usual config for remote_syslog
</match>
```

To get the general idea how truncation works, consider this table:

| Original Tag | Truncated tag |
|-------------|----------------|
| `kube.demo.test.test`                         | `demo.test.test`                    |
| `kube.demo.nginx-65899c769f-5zj6d.nginx`      | `demo.nginx-65899c769f-5zj*.nginx`  |
| `kube.demo.test.nginx11111111._lablels.hello` | `demo.test.nginx11111111`           |

### I want to push logs to Humio

Humio speaks the elasticsearh protocol so configuration is pretty similar to Elasticsearch. The example bellow is based on https://github.com/humio/kubernetes2humio/blob/master/fluentd/docker-image/fluent.conf.

```xml
<match **>
  @type elasticsearch
  include_tag_key false

  host "YOUR_HOST"
  path "/api/v1/dataspaces/YOUR_NAMESPACE/ingest/elasticsearch/"
  scheme "https"
  port "443"

  user "YOUR_KEY"
  password ""

  logstash_format true

  reload_connections "true"
  logstash_prefix "fluentd:kubernetes2humio"
  buffer_chunk_limit 1M
  buffer_queue_limit 32
  flush_interval 1s
  max_retry_wait 30
  disable_retry_limit
  num_threads 8
</match>
```

### I want to push logs to papertrail

```xml
test.conf:
<match **>
    @type papertrail
    papertrail_host YOUR_HOST.papertrailapp.com
    papertrail_port YOUR_PORT
    flush_interval 30
</match>
```

### I want to push logs to an ELK cluster

```xml
<match ***>
  @type elasticsearch
  host ...
  port ...
  index_name ...
  # many options available
</match>
```

For details you should consult the plugin documentation.

### I want to validate my config file before using it as a configmap

The container comes with a file validation command. To use it put all your \*.conf file in a directory. Use the namespace name for the filename. Then use this one-liner, bind-mounting the folder and feeding it as a `DATASOURCE_DIR` env var:


```bash
docker run --entrypoint=/bin/validate-from-dir.sh \
    --net=host --rm \
    -v /path/to/config-folder:/workspace \
    -e DATASOURCE_DIR=/workspace \
    jvassev/kube-fluentd-operator:latest
```

It will run fluentd in dry-run mode and even catch incorrect plug-in usage.
This is so common that it' already captured as a script [validate-logging-config.sh](https://github.com/vmware/kube-fluentd-operator/blob/master/config-reloader/validate-logging-config.sh).
The preferred way to use it is to copy it to your project and invoke it like this:

```bash
validate-logging-config.sh path/to/folder

```

All `path/to/folder/*.conf` files will be validated. Check stderr and the exit code for errors.

### I want to use Fluentd @label to simplify processing

Use `<label>` as usual, the daemon ensures that label names are unique cluster-wide. For example to route several pods' logs to destination X, and ignore a few others you can use this:

```xml
<match $labels(app=foo)>
  @type label
  @label blackhole
</match>

<match $labels(app=bar)>
  @type label
  @label blackhole
</match>

<label @blackhole>
  <match **>
    @type null
  </match>
</label>

# at this point, foo and bar's logs are being handled in the @blackhole chain,
# the rest are still available for processing
<match **>
  @type ..
</match>
```

### I want to parse ingress-nginx access logs and send them to a different log aggregator

The ingress controller uses a format different than the plain Nginx. You can use this fragment to configure the namespace hosting the ingress-nginx controller:

```xml
<filter $labels(app=nginx-ingress, _container=nginx-ingress-controller)>
  @type parser

  format /(?<remote_addr>[^ ]*) - \[(?<proxy_protocol_addr>[^ ]*)\] - (?<remote_user>[^ ]*) \[(?<time>[^\]]*)\] "(?<method>\S+)(?: +(?<request>[^\"]*) +\S*)?" (?<code>[^ ]*) (?<size>[^ ]*) "(?<referer>[^\"]*)" "(?<agent>[^\"]*)" (?<request_length>[^ ]*) (?<request_time>[^ ]*) \[(?<proxy_upstream_name>[^ ]*)\] (?<upstream_addr>[^ ]*) (?<upstream_response_length>[^ ]*) (?<upstream_response_time>[^ ]*) (?<upstream_status>[^ ]*)/
  time_format %d/%b/%Y:%H:%M:%S %z
  key_name log
  reserve_data true
</filter>

<match **>
  # send the parsed access logs here
</match>
```

The above configuration assumes you're using the Helm charts for Nginx ingress. If not, make sure to the change the `app` and `_container` labels accordingly. Given the horrendous regex above, you really should be outputting access logs in json format and just specify `format json`.

### I have my kubectl configured and my configmaps ready. I want to see the generated files before deploying the Helm chart

You need to run `make` like this:

```bash
make run-once
```

This will build the code, then `config-reloader` will connect to the K8S cluster, fetch the data and generate \*.conf files in the `./tmp` directory. If there are errors the namespaces will be annotated.

### I want to build a custom image with my own fluentd plugin

Use the `vmware/kube-fluentd-operator:TAG` as a base and do any modification as usual. If this plugin is not top-secret consider sending us a patch :)

### I run two clusters - in us-east-2 and eu-west-2. How to differentiate between them when pushing logs to a single location?

When deploying the daemonset using Helm, make sure to pass some metadata:

For the cluster in USA:

```bash
helm instal ... \
  --set=meta.key=cluster_info \
  --set=meta.values.region=us-east-2
```

For the cluster in Europe:

```bash
helm instal ... \
  --set=meta.key=cluster_info \
  --set=meta.values.region=eu-west-2
```

If you are using ELK you can easily get only the logs from Europe using `cluster_info.region: +eu-west-2`. In this example the metadata key is `cluster_info` but you can use any key you like.

### I don't want to annotate all my namespaces at all

It is possible to reduce configuration burden by using a default configmap name. The default value is `fluentd-config` - kube-fluentd-operator will read the configmap by that name if the namespace is not annotated.
If you don't like this default name or happen to use this configmap for other purposes then override the default with `--default-configmap=my-default`.

### How can I be sure to use a valid path for the .pos and .buf files

.pos files store the progress of the upload process and .buf are used for local buffering. Colliding .pos/.buf paths can lead to races in Fluentd. As such, `kube-fluentd-operator` tries hard to rewrite such path-based parameters in a predictable way. You only need to make sure they are unique for your namespace and `config-reloader` will take care to make them unique cluster-wide.

### I dont like the annotation name logging.csp.vmware.com/fluentd-configmap

Use `--annotation=acme.com/fancy-config` to use acme.com/fancy-config as annotation name. However, you'd also need to customize the Helm chart. Patches are welcome!

## Known issues

Currently space-delimited tags are not supported. For example, instead of `<filter a b>`, you need to use `<filter a>` and `<filter b>`.
This limitation will be addressed in a later version.


## Releases

* [CHANGELOG.md](CHANGELOG.md).

## Resoures

* This plugin is used to provide kubernetes metadata https://github.com/fabric8io/fluent-plugin-kubernetes_metadata_filter
* This daemonset definition is used as a template: https://github.com/fluent/fluentd-kubernetes-daemonset/tree/master/docker-image/v0.12/debian-elasticsearch, however `kube-fluentd-operator` uses version 1.x version of fluentd and all the compatible plugin versions.
* This [Github issue](https://github.com/fabric8io/fluent-plugin-kubernetes_metadata_filter/issues/73) was the inspiration for the project. In particular it borrows the tag rewriting based on Kubernetes metadata to allow easier routing after that.

## Contributing

The kube-fluentd-operator project team welcomes contributions from the community. If you wish to contribute code and you have not
signed our contributor license agreement (CLA), our bot will update the issue when you open a Pull Request. For any
questions about the CLA process, please refer to our [FAQ](https://cla.vmware.com/faq). For more detailed information,
refer to [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
