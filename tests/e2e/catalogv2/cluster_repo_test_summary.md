# `cluster_repo_test.go` Summary

Verifies that ClusterRepo resources can be created, updated, and deleted for HTTP, Git, and OCI repository types, including handling of rate limiting, multiple charts, tag filters, and enable/disable functionality.

## `TestHTTPRepo`
Starts an HTTP test server serving helm charts, then creates an HTTP ClusterRepo, updates it to a different URL, and deletes it.
- Checks the ClusterRepo is created and resources are downloaded from the initial HTTP URL.
- Checks updating the ClusterRepo URL to a different HTTP URL downloads new resources.
- Checks the updated ClusterRepo has an incremented ObservedGeneration.
- Checks deleting the ClusterRepo succeeds.

## `TestGitRepo`
Creates a Git ClusterRepo pointing to the rancher/charts repository, updates the URL to rancher/rke2-charts, and deletes it.
- Checks the ClusterRepo is created and resources are downloaded from the initial Git URL.
- Checks updating the ClusterRepo to a different Git URL downloads new resources.
- Checks the updated ClusterRepo has an incremented ObservedGeneration.
- Checks deleting the ClusterRepo succeeds.

## `TestGitRepoRetries`
Creates a Git ClusterRepo and verifies it handles retries when accessing the repository.
- Checks the ClusterRepo is created and reaches downloaded status.

## `TestOCIRepo`
Starts an OCI registry, pushes a testingchart, creates an OCI ClusterRepo with a chart reference, updates to a tag-specific URL, and deletes it.
- Checks the ClusterRepo is created and resources are downloaded from the initial OCI URL.
- Checks updating the ClusterRepo to a tag-specific OCI URL downloads new resources.
- Checks the updated ClusterRepo has an incremented ObservedGeneration.
- Checks deleting the ClusterRepo succeeds.

## `TestOCIRepo2`
Starts an OCI registry, pushes testingchart, creates an OCI ClusterRepo at repository root, updates to a broader path, and deletes it.
- Checks the ClusterRepo is created and resources are downloaded from the repository path URL.
- Checks updating to a root URL downloads resources.
- Checks deleting the ClusterRepo succeeds.

## `TestOCIRepo3`
Tests OCI ClusterRepo with 4xx HTTP errors (404, 401, 403) from the registry.
- Checks each 4xx error status code is recorded in the ClusterRepo condition message.
- Checks no ConfigMap is created for the failed repository.
- Checks the NumberOfRetries remains 0 for non-retryable errors.

## `TestOCIRepo4`
Tests OCI ClusterRepo with 429 (TooManyRequests) response from the registry without RateLimited-Remaining header.
- Checks the ClusterRepo initially fails with OCIDownloaded condition set to False.
- Checks after backoff retry, the ClusterRepo eventually succeeds with 2 charts (testingchart and testchart) in the ConfigMap index.
- Checks the NumberOfRetries is 0 after successful recovery.

## `TestOCIRepo5`
Tests OCI ClusterRepo with 429 response with RateLimited-Remaining header indicating rate limiting.
- Checks the ClusterRepo initially fails with OCIDownloaded condition set to False.
- Checks after backoff retry, the ClusterRepo succeeds with charts available.

## `TestOCIRepoMultipleChartRepos`
Starts an OCI registry with 300 test charts pushed, creates an OCI ClusterRepo, updates the URL, and deletes it.
- Checks the ClusterRepo is created with 300 charts from the registry.
- Checks updating the URL to a tag-specific version downloads resources.
- Checks deleting the ClusterRepo succeeds.

## `TestOCIRepoWithOptions`
Starts an OCI registry with testingchart versions 0.1.0 and 1.0.0, creates an OCI ClusterRepo with tag filter "< 1.0.0".
- Checks the ClusterRepo is created and only includes charts matching the tag filter.

## `TestOCIRepoChartInstallation`
Creates an OCI ClusterRepo, installs a chart from it, verifies the App resource has the correct label, and uninstalls it.
- Checks the ClusterRepo is created and downloads resources.
- Checks the chart can be installed.
- Checks the installed App has the `catalog.cattle.io/cluster-repo-name` label set to the repository name.
- Checks the chart can be uninstalled.
- Checks deleting the ClusterRepo succeeds.

## `TestOCIEnableRepo`
Creates an OCI ClusterRepo, disables it (so updates are not downloaded), adds a new chart to the registry, re-enables the ClusterRepo, and verifies only the original chart exists when disabled and both exist when enabled.
- Checks the ClusterRepo is initially created and downloads resources.
- Checks disabling the ClusterRepo prevents new charts from being discovered when the registry is updated.
- Checks the ConfigMap contains only 1 chart while disabled.
- Checks enabling the ClusterRepo and forcing refresh picks up the new chart.
- Checks the ConfigMap contains 2 charts after re-enabling.
