# Development

This doc explains how to setup a development environment so you can get started
[contributing](https://github.com/knative/docs/blob/master/community/CONTRIBUTING.md)
to Knative `pkg`. Also take a look at:

- [The pull request workflow](https://github.com/knative/docs/blob/master/community/CONTRIBUTING.md#pull-requests)

## Getting started

1. Create [a GitHub account](https://github.com/join)
1. Setup
   [GitHub access via SSH](https://help.github.com/articles/connecting-to-github-with-ssh/)
1. Install [requirements](#requirements)
1. Set up your [shell environment](#environment-setup)
1. [Create and checkout a repo fork](#checkout-your-fork)

Before submitting a PR, see also [CONTRIBUTING.md](./CONTRIBUTING.md).

### Requirements

You must install these tools:

1. [`go`](https://golang.org/doc/install): The language Knative `pkg` is built
   in
1. [`git`](https://help.github.com/articles/set-up-git/): For source control
1. [`dep`](https://github.com/golang/dep): For managing external dependencies.

### Environment setup

To get started you'll need to set these environment variables (we recommend
adding them to your `.bashrc`):

1. `GOPATH`: If you don't have one, simply pick a directory and add
   `export GOPATH=...`
1. `$GOPATH/bin` on `PATH`: This is so that tooling installed via `go get` will
   work properly.

`.bashrc` example:

```shell
export GOPATH="$HOME/go"
export PATH="${PATH}:${GOPATH}/bin"
```

### Checkout your fork

The Go tools require that you clone the repository to the
`src/github.com/knative/pkg` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [fork of this repo](https://help.github.com/articles/fork-a-repo/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/github.com/knative
cd ${GOPATH}/src/github.com/knative
git clone git@github.com:${YOUR_GITHUB_USERNAME}/pkg.git
cd pkg
git remote add upstream git@github.com:knative/pkg.git
git remote set-url --push upstream no_push
```

_Adding the `upstream` remote sets you up nicely for regularly
[syncing your fork](https://help.github.com/articles/syncing-a-fork/)._

Once you reach this point you are ready to do a full build and deploy as
described below.
