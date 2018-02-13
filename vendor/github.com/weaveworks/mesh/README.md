# mesh [![GoDoc](https://godoc.org/github.com/weaveworks/mesh?status.svg)](https://godoc.org/github.com/weaveworks/mesh) [![Circle CI](https://circleci.com/gh/weaveworks/mesh.svg?style=svg)](https://circleci.com/gh/weaveworks/mesh)

Mesh is a tool for building distributed applications.

Mesh implements a [gossip protocol](https://en.wikipedia.org/wiki/Gossip_protocol)
that provide membership, unicast, and broadcast functionality
with [eventually-consistent semantics](https://en.wikipedia.org/wiki/Eventual_consistency).
In CAP terms, it is AP: highly-available and partition-tolerant.

Mesh works in a wide variety of network setups, including thru NAT and firewalls, and across clouds and datacenters.
It works in situations where there is only partial connectivity,
 i.e. data is transparently routed across multiple hops when there is no direct connection between peers.
It copes with partitions and partial network failure.
It can be easily bootstrapped, typically only requiring knowledge of a single existing peer in the mesh to join.
It has built-in shared-secret authentication and encryption.
It scales to on the order of 100 peers, and has no dependencies.

## Using

Mesh is currently distributed as a Go package.
See [the API documentation](https://godoc.org/github.com/weaveworks/mesh).

We plan to offer Mesh as a standalone service + an easy-to-use API.
We will support multiple deployment scenarios, including
 as a standalone binary,
 as a container,
 as an ambassador or [sidecar](http://blog.kubernetes.io/2015/06/the-distributed-system-toolkit-patterns.html) component to an existing container,
 and as an infrastructure service in popular platforms.

## Developing

Mesh builds with the standard Go tooling. You will need to put the
repository in Go's expected directory structure; i.e.,
`$GOPATH/src/github.com/weaveworks/mesh`.

### Building

If necessary, you may fetch the latest version of all of the dependencies into your GOPATH via

`go get -d -u -t ./...`

Build the code with the usual

`go install ./...`

### Testing

Assuming you've fetched dependencies as above,

`go test ./...`

### Dependencies

Mesh is a library, designed to be imported into a binary package. 
Vendoring is currently the best way for binary package authors to ensure reliable, reproducible builds. 
Therefore, we strongly recommend our users use vendoring for all of their dependencies, including Mesh. 
To avoid compatibility and availability issues, Mesh doesn't vendor its own dependencies, and doesn't recommend use of third-party import proxies.

There are several tools to make vendoring easier, including
 [gb](https://getgb.io),
 [gvt](https://github.com/filosottile/gvt),
 [glide](https://github.com/Masterminds/glide), and
 [govendor](https://github.com/kardianos/govendor).

### Workflow

Mesh follows a typical PR workflow.
All contributions should be made as pull requests that satisfy the guidelines, below.

### Guidelines

- All code must abide [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Names should abide [What's in a name](https://talks.golang.org/2014/names.slide#1)
- Code must build on both Linux and Darwin, via plain `go build`
- Code should have appropriate test coverage, invoked via plain `go test`

In addition, several mechanical checks are enforced.
See [the lint script](/lint) for details.

