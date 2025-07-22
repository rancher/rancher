# Gotools

This directory contains Go-based tools to use with [go
tool](https://tip.golang.org/doc/modules/managing-dependencies#tools).

Each tool is within its own directory with its own `go.mod` file to avoid
dependency conflicts.

## Managing tools

**Using a tool**

```sh
go tool -modfile <path to modfile> <tool>
```

For example, to use controller-gen:

```sh
go tool -modfile gotools/controller-gen/go.mod controller-gen -h
```

**Add a new tool**

From repository root:

```sh
TOOLNAME=<tool name>
mkdir -p gotools/"$TOOLNAME"
go mod init -modfile=gotools/"$TOOLNAME"/go.mod github.com/rancher/rancher/gotools/"$TOOLNAME"
go get -tool -modfile=gotools/"$TOOLNAME"/go.mod <module>@<version>
```

For example, controller-gen was added this way:

```
TOOLNAME=controller-gen
mkdir -p gotools/"$TOOLNAME"
go mod init -modfile=gotools/"$TOOLNAME"/go.mod github.com/rancher/rancher/gotools/"$TOOLNAME"
go get -tool -modfile=gotools/"$TOOLNAME"/go.mod sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.1
```


**Update existing tool**

From repository root:

```sh
TOOLNAME=<tool name>
go get -tool -modfile=gotools/"$TOOLNAME"/go.mod <module>@<new version>
```

For example, to update controller-gen to v0.17.3:

```
TOOLNAME=controller-gen
go get -tool -modfile=gotools/"$TOOLNAME"/go.mod sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.3
```
