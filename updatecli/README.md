# updatecli automation

The Rancher project uses [updatecli] to automate and orchestrate security
related updates and versions bumps.

## Tool

We use updatecli for this automation, instead of Dependabot or Renovate,
because of its extensibility and multiple plugins resources that allow greater
flexibility when automating sequences of conditional update steps.

For detailed information on how to use updatecli, please consult its
[documentation].

## Scheduled workflow

The automation runs as a GitHub Actions scheduled workflow once per day. Manual
execution of the pipelines can be [triggered] when needed.

## Project organization

A manifest or pipeline consists of three stages: `sources`, `conditions` and
`targets`, that define how to apply the update strategy.

When adding a new manifest, please follow the example structure defined below.

```
updatecli/
├── README.md
├── scripts                                # For auxiliary scripts if needed
├── updatecli.d                            # For the update related workflows
│   ├── update-k8s-k3s                     # Each workflow should have its own subdirectory
│   └── update-versions-config-yaml        # Another workflow in its own directory
└── values.d                               # For variable related configuration files
    ├── values.yaml                        # Configuration values
    └── versions.yaml                      # Configuration versions
```

The manifest files must be placed inside a directory path named accordingly to
its main purpose.

## Local testing

Local testing of manifests require:

1. The updatecli binary that can be downloaded from
[updatecli/updatecli#releases]. Test only with the latest stable version.
   1. Always run locally with the command `diff`, that will show the changes
without actually applying them.
1. A GitHub personal fine-grained token.
   1. For obvious security reasons and to avoid leaking your GH PAT, export it
as a local environment variable.

```shell
export UPDATECLI_GITHUB_TOKEN="your GH token"
updatecli diff --clean --values updatecli/values.d/values.yaml --values <other values files> --config updatecli/updatecli.d/<your workflow> 
```

## Contributing

Before contributing, please follow the guidelines provided in this README and
make sure to test locally your changes, and against your own fork, before
opening a PR.


<!-- Links -->
[updatecli]: https://github.com/updatecli/updatecli
[documentation]: https://www.updatecli.io/docs/prologue/introduction/
[triggered]: https://github.com/rancher/rancher/actions/workflows/updatecli.yml
[updatecli/updatecli#releases]: https://github.com/updatecli/updatecli/releases
