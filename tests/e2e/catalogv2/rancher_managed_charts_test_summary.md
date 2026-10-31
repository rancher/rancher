# `rancher_managed_charts_test.go` Summary

Verifies that Rancher's system-managed `rancher-aks-operator` chart tracks the latest available version from the `rancher-charts` repo, and correctly handles upgrades, degraded/working-version transitions, and icon serving for bundled catalogs.

## `TestInstallChartLatestVersion`
**Arrange:**
- Points the `rancher-charts` ClusterRepo at `charts-small-fork`'s `aks-integration-test-working-charts` branch (a controlled chart source with known versions) and waits for it to download.

**Act:** Enables AKS on the management cluster (sets `AKSConfig`).

**Assert:**
- Checks the `rancher-aks-operator` app reaches `StatusDeployed`.
- Checks the deployed chart version is `104.0.2+up1.9.0`.
- Checks this version matches the latest version available from the catalog.
- Checks no explicit Helm values are set on the app or chart.

## `TestUpgradeChartToLatestVersion`
**Arrange:**
- Points the `rancher-charts` ClusterRepo at `charts-small-fork`'s `aks-integration-test-working-charts` branch and waits for it to download.

**Act 1:** Removes the top (truly-latest) `rancher-aks-operator` entry from the downloaded index ConfigMap, then enables AKS on the management cluster.
**Assert 1:**
- Checks `rancher-aks-operator` deploys at the now-highest remaining version (`104.0.1+up1.9.0`), which is lower than the actual latest version recorded before the edit.
- Checks no explicit Helm values are set.

**Act 2:** Reverts the index ConfigMap to its original content and forces the ClusterRepo to refresh.
**Assert 2:**
- Checks `rancher-aks-operator` is automatically upgraded to the restored (true) latest version.
- Checks no explicit Helm values are set.

## `TestUpgradeToWorkingVersion`
**Arrange:**
- Confirms the cluster has no `AKSConfig` and no `rancher-aks-charts` app yet.
- Points the `rancher-charts` ClusterRepo at `charts-small-fork`'s `aks-integration-test-1` branch (whose second-highest `rancher-aks-operator` version is deliberately broken) and waits for it to download.
- Records the current operation count for `rancher-aks-operator`.

**Act 1:** Removes the newest index entry (promoting the broken version to "latest"), then enables AKS on the management cluster.
**Assert 1:**
- Checks `rancher-aks-operator` reaches `StatusFailed`.
- Checks no explicit Helm values are set.
- Checks no more than 2 additional operations were created beyond the pre-recorded count (Rancher doesn't retry runaway on repeated failures).

**Act 2:** Reverts the index ConfigMap to its original content and forces the ClusterRepo to refresh.
**Assert 2:**
- Checks `rancher-aks-operator` eventually reaches `StatusDeployed` at the restored (true) latest version.

## `TestUpgradeToBrokenVersion`
**Arrange:**
- Points the `rancher-charts` ClusterRepo at `charts-small-fork`'s `aks-integration-test-2` branch and waits for it to download.

**Act 1:** Removes the newest index entry (forcing fallback to a lower, working version), then enables AKS on the management cluster.
**Assert 1:**
- Checks `rancher-aks-operator` deploys successfully at the resulting version (`102.0.0+up1.1.0`).
- Checks no explicit Helm values are set.

**Act 2:** Reverts the index ConfigMap to restore the true (broken) latest version entry and forces the ClusterRepo to refresh.
**Assert 2:**
- Checks `rancher-aks-operator` transitions to `StatusFailed` at the restored version.
- Checks no more than 2 additional operations were created between the successful and failed states (bounded retries).

## `TestServeIcons`
**Arrange:**
- Clones the `charts-small-fork` repo locally into the directory Rancher expects for a prebuilt/bundled catalog.
- Creates a ClusterRepo pointing at the same fork (`main` branch) and waits for it to download; confirms more than 1 chart is discoverable and that the `system-catalog` setting starts as `external`.

**Act:** Updates the `system-catalog` setting to `bundled`.

**Assert:**
- Checks the setting updates to `bundled`.
- Checks fetching the `rancher-compliance` chart icon (served via the `file://`-backed bundled catalog) succeeds and returns non-empty image data.
