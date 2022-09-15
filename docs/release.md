# Releasing Rancher

This page should include everything needed to create a release candidate or release.

## Check chart and KDM sources

Run `make check-chart-kdm-source-values` with the environment variable `RELEASE_TYPE` set to the type of release you are trying to create.

For release candidate:

```
RELEASE_TYPE=rc make check-chart-kdm-source-values
```

For final rc and release:

```
RELEASE_TYPE=final-rc make check-chart-kdm-source-values
```

There should be no lines showing `INCORRECT`. You can use `make change-final-rc-values` to set the values correctly.

For release candidate:

```
RELEASE_ACTION=revert make change-final-rc-values
```

For final rc and release:

```
RELEASE_ACTION=release make change-final-rc-values
```

Run `make check-chart-kdm-source-values` with `RELEASE_TYPE` correctly set to check the values again.

## Check open PRs for versions bumps

Pull requests are created automatically when versions of components that Rancher uses are tagged. You can [list open pull requests that have been automatically created](https://github.com/rancher/rancher/pulls?q=is%3Apr+label%3Astatus%2Fauto-created+is%3Aopen), and make sure they are merged. 

## Check for Go module updates

Run `make list-gomod-updates` to list all Rancher Go modules that have updated versions available. Create pull requests to update Go modules using the GitHub Actions workflow [Go Get](https://github.com/rancher/rancher/actions/workflows/go-get.yml)

Example output:

```
github.com/rancher/apiserver needs update (have: v0.0.0-20220223185512-c4e289f92e46, available: v0.0.0-20220513144301-4808910b5d4d)
github.com/rancher/dynamiclistener needs update (have: v0.3.1-0.20210616080009-9865ae859c7f, available: v0.3.3)
github.com/rancher/fleet/pkg/apis needs update (have: v0.0.0-20210918015053-5a141a6b18f0, available: v0.0.0-20220521053957-26e7c98cdb47)
github.com/rancher/lasso needs update (have: v0.0.0-20220412224715-5f3517291ad4, available: v0.0.0-20220519004610-700f167d8324)
github.com/rancher/lasso/controller-runtime needs update (have: v0.0.0-20220303220250-a429cb5cb9c9, available: v0.0.0-20220519004610-700f167d8324)
github.com/rancher/machine needs update (have: v0.15.0-rancher86, available: v0.15.0-rancher9)
github.com/rancher/norman needs update (have: v0.0.0-20220517230400-5a324b6fc6b1, available: v0.0.0-20220520225714-4cc2f5a97011)
github.com/rancher/rdns-server needs update (have: v0.0.0-20180802070304-bf662911db6a, available: v0.5.8)
github.com/rancher/security-scan needs update (have: v0.1.7-0.20200222041501-f7377f127168, available: v0.2.7)
github.com/rancher/steve needs update (have: v0.0.0-20220415184129-b23977e7f1b5, available: v0.0.0-20220503004032-53511a06ff37)
github.com/rancher/system-upgrade-controller/pkg/apis needs update (have: v0.0.0-20210727200656-10b094e30007, available: v0.0.0-20220502195742-7eb95a99eb25)
github.com/rancher/wrangler needs update (have: v0.8.11-0.20220411195911-c2b951ab3480, available: v1.0.0)
```

## Create tag and push

In this example, the tag is v2.6.6-rc1

```
cd rancher
git remote add rancher-release git@github.com:rancher/rancher.git
git checkout rancher-release/release/v2.6
git tag v2.6.6-rc1
git push rancher-release v2.6.6-rc1
```

Wait for the Drone build to complete, see [Drone Publish](https://drone-publish.rancher.io/rancher/rancher) for status.

## Start the Drone build for image scanning

When the Drone build has completed, start a [Drone build for image scanning](https://github.com/rancher/image-scanning/#triggering-a-scan).

# Additional steps for final release candidate

These are the steps required for creating a final release candidate. The final release candidate is a copy of the actual release, except that we tag it as v2.X.X-rcX. All components and configuration should have a non-rc tag, and point to the release branch. (`release-v2.X` instead of `dev-v2.X`)

## Check for rc components

Check the previous release candidate on the [GitHub releases page](https://github.com/rancher/rancher/releases). It should show all the rc components found in that release candidate, make sure they are corrected before tagging the final release candidate. The file used for this information for each release (candidate) is `rancher-components.txt`.

# Additional steps for release

## Create the release tag and push

Make sure the branch is up-to-date with the remote, in this example, the branch is `release/v2.6` and the release tag is `v2.6.6`

```
cd rancher
git remote add rancher-release git@github.com:rancher/rancher.git
git checkout rancher-release/release/v2.6
git push rancher-release v2.6.6
```

Wait for the Drone build to complete, see [Drone Publish](https://drone-publish.rancher.io/rancher/rancher) for status.

## Promote Docker image to latest

Run `./scripts/promote-docker-image.sh` with the created tag as first argument and `latest` as second argument. In this example, the tag is `v2.6.6`.

```
./scripts/promote-docker-image.sh v2.6.6 latest
```

## Add release notes to GitHub release page

Add the release notes to the GitHub release found on the [GitHub releases page](https://github.com/rancher/rancher/releases).

## Run release check script

Run `make post-release-checks` with environment variable `POSTRELEASE_RANCHER` set to the created tag. In this example, the tag is `v2.6.6`.

```
POSTRELEASE_RANCHER_VERSION=v2.6.6 make post-release-checks
```

This will check Docker images `rancher/rancher:v2.6.6`, `rancher/rancher:latest`, and check the `rancher-latest` Helm chart repository.

## Post an announcement on Rancher Forums

Go to https://forums.rancher.com/c/announcements/12 and create a new topic for the release with the release notes.

## Update README.md in `rancher/rancher` repository

Create a pull request to update the README using the GitHub Actions workflow [Update README](https://github.com/rancher/rancher/actions/workflows/update-readme.yml).

# Additional steps for promoting a release to stable

## Promote Docker image to stable

Run `./scripts/promote-docker-image.sh` with the created tag as first argument and `stable` as second argument. In this example, the tag is `v2.6.6`.

```
./scripts/promote-docker-image.sh v2.6.6 stable
```

## Promote Helm chart to stable

Run `./scripts/chart/promote-to-stable.sh` with the created tag as first argument. In this example, the tag is `v2.6.6`.

```
./scripts/chart/promote-to-stable.sh v2.6.6
```

## Run release check script

Run `make post-release-checks` with environment variable `POSTRELEASE_RANCHER` set to the created tag and `POSTRELEASE_RANCHER_STABLE` set to `true`. In this example, the tag is `v2.6.6`.

```
POSTRELEASE_RANCHER_VERSION=v2.6.6 POSTRELEASE_RANCHER_STABLE=true make post-release-checks
```

## Update README.md in `rancher/rancher` repository

Create a pull request to update the README using the GitHub Actions workflow [Update README](https://github.com/rancher/rancher/actions/workflows/update-readme.yml).
