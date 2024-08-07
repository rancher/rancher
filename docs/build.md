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

Then update the [export-config](../scripts/export-config) script.

```
CATTLE_RANCHER_WEBHOOK_VERSION=$(yq -e '.webhookVersion' "$file")
export CATTLE_RANCHER_WEBHOOK_VERSION
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
- [Dockerfile.agent](../package/Dockerfile.agent)
- [pkg/settings/setting.go](../pkg/settings/setting.go)

### The build.yaml file

It's better to follow the standard Kubernetes convention of preferring camelCase keys in the YAML file.

The exported resulting environment variables should be like standard ENV_VARS.
