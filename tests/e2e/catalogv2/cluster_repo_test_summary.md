# `cluster_repo_test.go` Summary

Verifies that ClusterRepo resources of HTTP, Git, and OCI type support the create/update/delete lifecycle, including retry/backoff behavior on download failures, 4xx and 429 error handling, OCI tag filtering, and enabling/disabling to control chart discovery.

## `TestHTTPRepo`
**Arrange:**
- Starts an HTTP test server serving Helm charts from local testdata.

**Act 1:** Creates an HTTP ClusterRepo pointing at the test server's URL.
**Assert 1:**
- Checks the ClusterRepo downloads resources and its status URL matches the test server URL.

**Act 2:** Updates the ClusterRepo's URL to the public stable Rancher HTTP repo.
**Assert 2:**
- Checks new resources are downloaded (DownloadTime changes) and the status URL matches the new URL.
- Checks `ObservedGeneration` increases.

**Act 3:** Deletes the ClusterRepo.
**Assert 3:**
- Checks fetching the ClusterRepo by ID now returns an error.

## `TestGitRepo`
**Arrange:**
- None.

**Act 1:** Creates a Git ClusterRepo pointing at `rancher/charts`.
**Assert 1:**
- Checks the ClusterRepo downloads resources and its status URL matches `rancher/charts`.

**Act 2:** Updates the ClusterRepo's Git URL to `rancher/rke2-charts`.
**Assert 2:**
- Checks new resources are downloaded and the status URL matches the new repo.
- Checks `ObservedGeneration` increases.

**Act 3:** Deletes the ClusterRepo.
**Assert 3:**
- Checks fetching the ClusterRepo by ID now returns an error.

## `TestGitRepoRetries`
**Arrange:**
- None.

**Act 1:** Creates a Git ClusterRepo pointing at `charts-small-fork` with an invalid branch name and a backoff configuration of 30s min wait, 60s max wait, and 2 max retries.
**Assert 1:**
- Checks the `RepoDownloaded` condition stays `False`, with `NumberOfRetries` incrementing on each attempt up to the configured max (2) before resetting to 0, confirming retries are exhausted rather than retried indefinitely.

**Act 2:** Updates the ClusterRepo's Git branch to the valid `main` branch.
**Assert 2:**
- Checks the ClusterRepo downloads successfully (DownloadTime advances past the pre-fix value).

**Act 3:** Deletes the ClusterRepo.
**Assert 3:**
- Checks fetching the ClusterRepo by ID now returns an error.

## `TestOCIRepo`
**Arrange:**
- Starts a local OCI registry and pushes `testingchart` version `0.1.0` to it.

**Act 1:** Creates an OCI ClusterRepo pointing at the chart's registry path (no tag).
**Assert 1:**
- Checks the ClusterRepo downloads resources and its status URL matches the registry path.

**Act 2:** Updates the ClusterRepo's URL to the same path with an explicit `:0.1.0` tag.
**Assert 2:**
- Checks new resources are downloaded and the status URL matches the tagged URL.
- Checks `ObservedGeneration` increases.

**Act 3:** Deletes the ClusterRepo.
**Assert 3:**
- Checks fetching the ClusterRepo by ID now returns an error.

## `TestOCIRepo2`
**Arrange:**
- Starts a local OCI registry and pushes `testingchart` version `0.1.0` to it.

**Act 1:** Creates an OCI ClusterRepo pointing at the registry's `rancher` repository path.
**Assert 1:**
- Checks the ClusterRepo downloads resources and its status URL matches the repository path.

**Act 2:** Updates the ClusterRepo's URL to the broader registry root.
**Assert 2:**
- Checks new resources are downloaded and the status URL matches the root URL.

**Act 3:** Deletes the ClusterRepo.
**Assert 3:**
- Checks fetching the ClusterRepo by ID now returns an error.

## `TestOCIRepo3`
**Arrange:**
- None.

**Act:** Creates an OCI ClusterRepo pointing at a registry that returns a 4xx status, tried in turn for 404, 401, and 403.

**Assert:**
- Checks the `RepoDownloaded` condition becomes `False` with a message of `error <code>: <message>` for each status code.
- Checks `NumberOfRetries` stays 0 (non-retryable errors aren't retried).
- Checks no index ConfigMap is created for the failed repository.

## `TestOCIRepo4`
**Arrange:**
- None.

**Act:** Creates an OCI ClusterRepo (max 1 retry) against a registry that returns 429 (Too Many Requests) on the `testchart` manifest fetch, without a `RateLimit-Remaining` header.

**Assert:**
- Checks the initial download attempt exhausts its retry and leaves the `RepoDownloaded` condition `False` with `NumberOfRetries` reset to 0.
- Checks that once the server's internal rate-limit window resets, the same ClusterRepo re-downloads successfully, with the index ConfigMap containing both `testingchart` and `testchart` entries (2 each) with populated digests.
- Checks `NumberOfRetries` is 0 after the successful recovery.

## `TestOCIRepo5`
**Arrange:**
- None.

**Act:** Creates an OCI ClusterRepo against a registry that returns 429 on the `testchart` manifest fetch, this time including a `RateLimit-Remaining` header.

**Assert:**
- Checks the `RepoDownloaded` condition is initially `False`.
- Checks that after the backoff/retry period, the ClusterRepo succeeds with both charts available in the index.

## `TestOCIRepoMultipleChartRepos`
**Arrange:**
- Starts an OCI registry and pushes 300 separate chart repositories (`testingchart-0` .. `testingchart-299`), each with version `0.1.0`.

**Act 1:** Creates an OCI ClusterRepo pointing at `testingchart-0` (no tag).
**Assert 1:**
- Checks the ClusterRepo downloads resources and its status URL matches the registry path.

**Act 2:** Updates the ClusterRepo's URL to the same path with an explicit `:0.1.0` tag.
**Assert 2:**
- Checks new resources are downloaded and the status URL matches the tagged URL.

**Act 3:** Deletes the ClusterRepo.
**Assert 3:**
- Checks fetching the ClusterRepo by ID now returns an error.

## `TestOCIRepoWithOptions`
**Arrange:**
- Starts an OCI registry and pushes `testingchart` versions `0.1.0` and `1.0.0`.

**Act 1:** Creates an OCI ClusterRepo with `OCIOptions.TagFilter` set to `< 1.0.0`.
**Assert 1:**
- Checks the downloaded index only contains `testingchart` entries satisfying the filter (just `0.1.0`).

**Act 2:** Updates the ClusterRepo to set `DownloadAllTags = true` while keeping the same tag filter.
**Assert 2:**
- Checks the index still contains only 1 matching `testingchart` entry (the tag filter is still enforced).

**Act 3:** Updates the ClusterRepo again, clearing the tag filter while keeping `DownloadAllTags = true`.
**Assert 3:**
- Checks the index now contains both `testingchart` entries (`0.1.0` and `1.0.0`).

**Act 4:** Deletes the ClusterRepo.
**Assert 4:**
- Checks fetching the ClusterRepo by ID now returns an error.

## `TestOCIRepoChartInstallation`
**Arrange:**
- Starts an OCI registry and pushes `testingchart` version `0.1.0`.

**Act 1:** Creates an OCI ClusterRepo (`oci`) pointing at the registry.
**Assert 1:**
- Checks the ClusterRepo downloads successfully.

**Act 2:** Installs `testingchart` from the repo as release `testreleasename` in the `default` namespace.
**Assert 2:**
- Checks the App reaches `StatusDeployed`.
- Checks the installed App carries a `catalog.cattle.io/cluster-repo-name` label equal to `oci`.

**Act 3:** Uninstalls the `testreleasename` release.
**Assert 3:**
- Checks the App resource is deleted (a Delete event is observed).

**Act 4:** Deletes the ClusterRepo.
**Assert 4:**
- Checks deleting it again returns an error (already gone).

## `TestOCIEnableRepo`
**Arrange:**
- Starts an OCI registry, pushes `testingchart`, creates an OCI ClusterRepo (`oci`) pointing at it, and confirms the initial download.

**Act 1:** Disables the ClusterRepo (`Spec.Enabled = false`), adds a second chart (`testchart`) to the registry, and forces a refresh.
**Assert 1:**
- Checks the index ConfigMap still contains only the original chart (1 entry) — the new chart isn't discovered while disabled.

**Act 2:** Re-enables the ClusterRepo and forces another refresh.
**Assert 2:**
- Checks the index ConfigMap now contains both charts (2 entries).
