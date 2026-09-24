# `charts_test.go` Summary

Verifies that charts can be installed, upgraded, and uninstalled with various toleration configurations (automatic, custom, and none) on tainted control plane nodes, and that schema validation can be skipped during chart operations.

## `TestInstallChartWithAutomaticTolerationOnTaintedCPNode`
Adds a custom taint to the control plane node, then installs the rancher-aks-operator-crd chart with automatic CP tolerations enabled.
- Checks the chart reaches StatusDeployed.
- Checks the installation operation pod includes the custom taint (testTaint) in its tolerations.

## `TestInstallChartWithCustomTolerationOnTaintedCPNode`
Adds a custom taint to the control plane node, then installs the rancher-aks-operator-crd chart with a custom toleration specified (testTaint key).
- Checks the chart reaches StatusDeployed.
- Checks the installation operation pod includes the custom taint (testTaint) in its tolerations.

## `TestUpgradeChartWithCustomTolerationOnTaintedCPNode`
Installs rancher-aks-operator-crd (version 104.0.1+up1.9.0) with custom toleration, adds a taint, then upgrades to version 104.0.2+up1.9.0 with custom toleration still specified.
- Checks the initial installation reaches StatusDeployed.
- Checks the initial operation pod includes the custom taint.
- Checks the upgrade reaches StatusDeployed.
- Checks the upgraded operation pod still includes the custom taint.

## `TestUpgradeChartWithAutomaticTolerationOnTaintedCPNode`
Installs rancher-aks-operator-crd (version 104.0.1+up1.9.0) with custom toleration, adds a taint, then upgrades to version 104.0.2+up1.9.0 with automatic CP tolerations enabled.
- Checks the initial installation reaches StatusDeployed.
- Checks the initial operation pod includes the custom taint.
- Checks the upgrade reaches StatusDeployed with automatic CP tolerations.
- Checks the upgraded operation pod includes the custom taint.

## `TestUpgradeChartInstalledWithoutTolerationsUsingAutomaticTolerations`
Installs rancher-aks-operator-crd (version 104.0.1+up1.9.0) without automatic CP tolerations, adds a taint to the control plane, then upgrades to version 104.0.2+up1.9.0 with automatic CP tolerations enabled.
- Checks the initial installation reaches StatusDeployed and includes default pod tolerations.
- Checks the upgrade reaches StatusDeployed with automatic CP tolerations.
- Checks the upgraded operation pod now includes the custom taint.

## `TestUninstallChartWithAutomaticTolerationOnTaintedCPNode`
Installs rancher-aks-operator-crd with custom toleration on a tainted node, then uninstalls with automatic CP tolerations enabled.
- Checks the installation reaches StatusDeployed.
- Checks the installation operation pod includes the custom taint.
- Checks the uninstall operation reaches StatusUninstalled.
- Checks the uninstall operation pod includes the custom taint.

## `TestUninstallChartWithCustomTolerationOnTaintedCPNode`
Installs rancher-aks-operator-crd with custom toleration on a tainted node, then uninstalls with the same custom toleration still specified.
- Checks the installation reaches StatusDeployed.
- Checks the installation operation pod includes the custom taint.
- Checks the uninstall operation reaches StatusUninstalled.
- Checks the uninstall operation pod includes the custom taint.

## `TestInstallChartWithSkipSchemaValidation`
Installs rancher-aks-operator-crd with schema validation skipped.
- Checks the chart reaches StatusDeployed.
- Checks the operation command includes `--skip-schema-validation=true`.

## `TestUpgradeChartWithSkipSchemaValidation`
Installs rancher-aks-operator-crd (version 104.0.2+up1.9.0), then upgrades the same chart with schema validation skipped.
- Checks the initial installation reaches StatusDeployed.
- Checks the upgrade reaches StatusDeployed.
- Checks the upgrade operation command includes `--skip-schema-validation=true`.
