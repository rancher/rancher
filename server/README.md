Developing/Testing Rancher Server Container
===========================================

The Rancher server container has in it multiple components.  It is often more efficient to run `rancher/server` using a master branch of one of the sub components than it is to build a whole new release of `rancher/server`.  One can use the `rancher/server:master` container to run master branch components.

REPOS environment variable
--------------------------

The REPOS environment variable is a space separated value of the repos/branchs that should be cloned, built, and ran as part of the `rancher/server` container.  Please note that for any repo besides `cattle`, `-v /var/run/docker.sock:/var/run/docker.sock` is required.

The REPOS environment variable has the format `GIT_URL[,BRANCH] GIT_URL[,BRANCH] ...`.  For example `REPOS=https://github.com/rancher/cattle,custom_branch`.  As a short hand, instead of the full git URL, you can put a component name and it will be assumed that the git URL is `https://github.com/rancher/COMPONENT`.  For example, `cattle` would be the same as `https://github.com/rancher/cattle`.

In the REPOS environment variable if `cattle` is not included it will be added automatically.  This means `cattle` is always ran from master.

Examples
--------

Run master container using cattle master only.
```
docker run -p 8080:8080 rancher/server:master
```


Run master container using cattle and rancher-compose-executor from master.
```
docker run -p 8080:8080 -e REPOS="rancher-compose-executor" -v /var/run/docker.sock:/var/run/docker.sock rancher/server:master
```

Run master using custom branch of websocket-proxy and rancher-compose-executor master.
```
docker run -p 8080:8080 -e REPOS="rancher-compose-executor https://github.com/ibuildthecloud/websocketproxy,fix-something" -v /var/run/docker.sock:/var/run/docker.sock rancher/server:master
```

Bind mounting
-------------

If you are developing and wish to bind mount source code instead of cloning from git then just bind mount to `/source/COMPONENT`.  For example

```
docker run -v /var/run/docker.sock:/var/run/docker.sock -v $(pwd)/rancher-compose-executor:/source/rancher-compose-executor -p 8080:8080 rancher/server:master
```

Don't include your repo in `REPOS` environment variable if you are bind mounting.

Restarting
----------

If you restart the container all code will be re-pulled and rebuilt.  For cattle is necesary to restart the container.  If you are bind mounting in source you can instead build on the host and then just kill your process (`killall rancher-compose-executor`).  Once the process dies cattle will restart it with the newly built binary.
