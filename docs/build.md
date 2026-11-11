## Build and package configuration

Build variables should be defined in a single file,
so that anyone who wants to build Rancher needs to only edit this file to change configuration and dependency versions.

Rancher relies on various subcomponents, such as the webhook.
These typically need to have set versions for Rancher to build and run properly.
Build variables can be used in different places and supplied to the applications in a variety of ways,
including as environment variables in Dockerfiles, constants in Go code, and so on.

The [build.yaml](../build.yaml) file is the single source of truth. It lists all values by name and value.
Changes to it should be committed to source control.

### Update an existing value

Edit the [build.yaml](../build.yaml) file and update the desired value. Run `go generate`. Commit any changes to source
control. To test locally, re-build Rancher with `make build` or re-package it with `make package`.

### Add a new value

To add a new value, do the following once.

Add it to [build.yaml](../build.yaml). For example:

```
webhookVersion: 2.0.6+up0.3.6-rc1
```

Run `go generate` from the root of the repo.

Now you can refer to the value wherever you need it.

#### Refer to the new value

If a new configuration value is an environment variable for a Dockerfile, capture it as an `ARG` and `ENV`. For example:

```
ARG CATTLE_FLEET_VERSION
ENV CATTLE_FLEET_VERSION=$CATTLE_FLEET_VERSION
```

Then pass it as via `docker build --build-arg MYVAR="$MYVAR" ...`

If a new configuration value is a regular string outside Dockerfiles, refer to the corresponding constant found in the
generated Go [file](../pkg/buildconfig/constants.go). For example:

```NewSetting("shell-image", buildconfig.DefaultShellVersion)```

The following are examples of files that often refer to newly added configuration values:

- [build-server](../scripts/build-server)
- [build-agent](../scripts/build-agent)
- [quick](../dev-scripts/quick)
- [Dockerfile](../package/Dockerfile)
- [Dockerfile.runtime](../Dockerfile.runtime)
- [pkg/settings/setting.go](../pkg/settings/setting.go)

### The build.yaml file

It's better to follow the standard Kubernetes convention of preferring camelCase keys in the YAML file.

The exported resulting environment variables should be like standard ENV_VARS.

### Chart images annotation

`go generate` also fills the `helm.sh/images` annotation of [chart/Chart.yaml](../chart/Chart.yaml) (see
[HIP-0015](https://github.com/helm/community/blob/main/hips/hip-0015.md)) from build.yaml. The list of images lives in
`pkg/codegen/buildconfig/chart_yaml_writer.go`.

The annotation lists the images used by the chart's own templates, plus the ones Rancher deploys at runtime:
`rancher-agent`, `system-agent`, `wins` and `machine`, and for Prime only `scc-operator`. Rancher has no subcharts, so entries do not use the
`dependency` field. Entries carry the standard `name` and `image` fields. The rancher and rancher-agent images keep the
`%APP_VERSION%` placeholder, which the chart build replaces, along with the `0.0.0-VERSION` chart version.

In the Prime repository, run `go generate` with `RANCHER_VERSION_TYPE=prime` to prefix the images with
`registry.rancher.com/`.

#### Generating for a copy of the chart

By default `go generate` updates `chart/` along with the other files derived from build.yaml. To generate for a build copy of the
chart instead, such as the Prime chart, set `CHART_DIR` to the directory of the copy. Only `Chart.yaml` and `values.yaml` of that
directory are updated, and the files of the repository are left untouched, so the repository stays clean. Keep the copy in a
git-ignored directory such as `build/`:

```shell
cp -r chart build/chart-prime
CHART_DIR=build/chart-prime RANCHER_VERSION_TYPE=prime scripts/generate-buildconfig
scripts/chart/build build/chart-prime
```
