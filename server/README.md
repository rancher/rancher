# Developing/Testing Rancher Server Container

The Rancher server container is comprised of multiple components (e.g. [cattle](https://github.com/rancher/cattle), [rancher-compose-executor](https://github.com/rancher/rancher-compose-executor), [websocket-proxy](https://github.com/rancher/websocket-proxy)). It is often more efficient to run `rancher/server` using a master branch of one of these sub components than it is to build a whole new release of `rancher/server`.  One can use the `rancher/server:master` container to run different components pointing to specific branches (including `master`) of the component instead having it pointed to a specific released version.

The list of components compiled in the Rancher server container and their current released  version can be found [here](https://github.com/rancher/cattle/blob/master/resources/content/cattle-global.properties).

```
* agent
* rancher-compose-executor
* catalog-service
* websocket-proxy
* go-machine-service
* rancher-auth-service
* secrets-api
```

## `REPOS` Environment Variable

The `REPOS` environment variable is a space separated value of the repositories/branches that should be cloned, built, and ran as part of the `rancher/server` container.

> **Note:** For any repo besides `cattle`, `-v /var/run/docker.sock:/var/run/docker.sock` is required as part of running the Rancher server container.

The value of `REPOS` environment variable supports two formats.

### Specific Branches

You can use any branch for a component with the following format, `GIT_URL[,BRANCH] GIT_URL[,BRANCH] ...`.  For example, `REPOS=https://github.com/rancher/cattle,custom_branch`. When testing personal forks or branches of components, this is the recommended format.

### Master Branch on the Rancher Repo

Instead of using the full git URL for the `master` branch, you can put a component name and it will default the git URL to `https://github.com/rancher/<COMPONENT>`.  For example, `cattle` would be the same as `https://github.com/rancher/cattle`.

> **Note:** In the `REPOS` environment variable, `cattle` will always be added automatically and run from the `master` branch.

## Examples

### Only `master` branch of cattle

Run the master container using the `master` branch of [cattle](https://github.com/rancher/cattle).

```
docker run -p 8080:8080 rancher/server:master
```

### Changing rancher-compose-executor to run from the `master` branch instead of the released version

Run the master container using the `master` branch of [cattle](https://github.com/rancher/cattle) and [rancher-compose-executor](https://github.com/rancher/rancher-compose-executor). Typically, rancher-compose-executor will use the specific released version that's in the [properties file in cattle](https://github.com/rancher/cattle/blob/master/resources/content/cattle-global.properties).

```
# Option 1:
docker run -p 8080:8080 -e REPOS="rancher-compose-executor" -v /var/run/docker.sock:/var/run/docker.sock rancher/server:master
# Option 2:
docker run -p 8080:8080 -e REPOS="https://github.com/rancher/rancher-compose-executor" -v /var/run/docker.sock:/var/run/docker.sock rancher/server:master
```

### Running custom multiple components

Run the master container using the `master` branch of [rancher-compose-executor](https://github.com/rancher/rancher-compose-executor) and a custom branch (i.e. `fix-something`) of [websocket-proxy](https://github.com/rancher/websocket-proxy).

```
docker run -p 8080:8080 -e REPOS="rancher-compose-executor https://github.com/ibuildthecloud/websocketproxy,fix-something" -v /var/run/docker.sock:/var/run/docker.sock rancher/server:master
```

## Bind mounting

If you are developing and wish to bind mount the source code instead of cloning from git, then just bind mount to `/source/COMPONENT`. For example:

```
docker run -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock -v $(pwd)/rancher-compose-executor:/source/rancher-compose-executor  rancher/server:master
```

> **Note:** Do **NOT** include your repo in the `REPOS` environment variable if you are bind mounting.

## Restarting

If you restart the master container, all code will be re-pulled and rebuilt.  For cattle, it is necessary to restart the container to get the latest.  If you are bind mounting in source code, you can build on the host, and then just kill your process (e.g. `killall rancher-compose-executor`).  Once the process dies, cattle will restart the process with the newly built binary.
