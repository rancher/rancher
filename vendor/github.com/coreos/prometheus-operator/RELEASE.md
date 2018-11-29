# How to cut a new release

> This guide is strongly based on the [Prometheus release instructions](https://github.com/prometheus/prometheus/wiki/HOWTO-cut-a-new-release).

## Branch management and versioning strategy

We use [Semantic Versioning](http://semver.org/).

We maintain a separate branch for each minor release, named `release-<major>.<minor>`, e.g. `release-1.1`, `release-2.0`.

The usual flow is to merge new features and changes into the master branch and to merge bug fixes into the latest release branch. Bug fixes are then merged into master from the latest release branch. The master branch should always contain all commits from the latest release branch.

If a bug fix got accidentally merged into master, cherry-pick commits have to be created in the latest release branch, which then have to be merged back into master. Try to avoid that situation.

Maintaining the release branches for older minor releases happens on a best effort basis.

## Prepare your release

For a patch release, work in the branch of the minor release you want to patch.

For a new major or minor release, create the corresponding release branch based on the master branch.

Bump the version in the `VERSION` file in the root of the repository. Once that's done, a number of files have to be re-generated, this is automated with the following make target:

```bash
$ make generate
```

Now that all version information has been updated, an entry for the new version can be added to the `CHANGELOG.md` file.

Entries in the `CHANGELOG.md` are meant to be in this order:

* `[CHANGE]`
* `[FEATURE]`
* `[ENHANCEMENT]`
* `[BUGFIX]`

Create a PR for the version and changelog changes to be reviewed.

## Draft the new release

Once the PR for the new release has been merged, make sure there is a release branch for the respective release. For new minor releases create the `release-<major>.<minor>` branch, for patch releases, merge  the master branch into the existing release branch. Should the release be a patch release for an older minor release, cherry-pick the respective changes.

Push the new or updated release branch to the upstream repository.

Tag the new release with a tag named `v<major>.<minor>.<patch>`, e.g. `v2.1.3`. Note the `v` prefix.

You can do the tagging on the commandline:

```bash
$ tag=$(< VERSION) && git tag -s "v${tag}" -m "v${tag}"
$ git push --tags
```

Signed tag with a GPG key is appreciated, but in case you can't add a GPG key to your Github account using the following [procedure](https://help.github.com/articles/generating-a-gpg-key/), you can replace the `-s` flag by `-a` flag of the `git tag` command to only annotate the tag without signing.

Our CI pipeline will automatically push a new docker image to quay.io.

Go to  https://github.com/coreos/prometheus-operator/releases/new, associate the new release with the before pushed tag, paste in changes made to `CHANGELOG.md` and click "Publish release".

Take a breath. You're done releasing.
