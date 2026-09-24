# `rancher_managed_charts_test.go` Summary

Verifies that Rancher-managed system charts are installed, upgraded, and downgraded correctly based on latest available versions, configuration map modifications, and system cluster updates.

## `TestInstallChartLatestVersion`
Updates the rancher-charts ClusterRepo to point to charts-small-fork on the aks-integration-test-working-charts branch, triggers cluster update, and verifies rancher-aks-operator deploys at the latest version.
- Checks the ClusterRepo is updated and resources are downloaded.
- Checks the rancher-aks-operator app reaches StatusDeployed.
- Checks the installed chart version is 104.0.2+up1.9.0.
- Checks the version matches the latest available from the catalog.
- Checks no values are set on the app.

## `TestUpgradeChartToLatestVersion`
Updates rancher-charts ClusterRepo, downgrades the latest version in the ConfigMap index, triggers cluster update, and verifies the system deploys the downgraded (older) version.
- Checks the ClusterRepo is updated and resources are downloaded.
- Checks the original latest version is extracted from the ConfigMap.
- Checks after downgrading the index, the app reaches StatusDeployed with the lower version.
- Checks the deployed version is less than the original latest.
- Checks the ConfigMap is reverted and ForceUpdate is triggered to recover the original index.
- Checks after recovery, the app reaches StatusDeployed with the original latest version.

## `TestUpgradeToWorkingVersion`
Sets the rancher-charts branch to aks-integration-test-1, removes the latest version from the ConfigMap index, triggers cluster update, and verifies degraded version is deployed, then reverts.
- Checks the ClusterRepo initially has no rancher-aks-charts app.
- Checks after modifying the ConfigMap and updating the cluster, the rancher-aks-operator app reaches StatusFailed.
- Checks the operation count for rancher-aks-operator is tracked before and after.
- Checks no more than 2 additional operations are created after the failed deployment.
- Checks after reverting the ConfigMap and forcing refresh, the app reaches StatusDeployed.

## `TestUpgradeToBrokenVersion`
Sets the rancher-charts branch to aks-integration-test-2, removes the latest version from the ConfigMap, triggers cluster update to deploy a broken version, then reverts.
- Checks after modifying the ConfigMap, the rancher-aks-operator app reaches StatusDeployed with version 102.0.0+up1.1.0.
- Checks the operation count is tracked before revert.
- Checks after reverting the ConfigMap and triggering ForceUpdate, the app reaches StatusFailed.
- Checks no more than 2 additional operations are created between successful and failed states.

## `TestServeIcons`
Clones the rancher/charts-small-fork repository to a specific build directory to test that Rancher serves chart icons from the prebuild helm repository location.
- Checks the clone directory is created.
