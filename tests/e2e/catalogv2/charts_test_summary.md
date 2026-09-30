# `charts_test.go` Summary

Verifies that charts can be installed, upgraded, and uninstalled with automatic, custom, or no CP-node tolerations on tainted control plane nodes, and that schema validation can be skipped during chart install/upgrade.

## `TestInstallChartWithAutomaticTolerationOnTaintedCPNode`
**Arrange:**
- Adds a custom taint (`testTaint`) to a control plane node.

**Act:** Installs the `rancher-aks-operator-crd` chart with `AutomaticCPTolerations` enabled.

**Assert:**
- Checks the chart reaches `StatusDeployed`.
- Checks the install operation's pod tolerations include the custom taint.

## `TestInstallChartWithCustomTolerationOnTaintedCPNode`
**Arrange:**
- Adds a custom taint (`testTaint`) to a control plane node.

**Act:** Installs the chart with an explicit `OperationTolerations` entry matching the taint (no automatic CP tolerations).

**Assert:**
- Checks the chart reaches `StatusDeployed`.
- Checks the install operation's pod tolerations include the custom taint.

## `TestUpgradeChartWithCustomTolerationOnTaintedCPNode`
**Arrange:**
- Adds a custom taint to a control plane node.
- Installs `rancher-aks-operator-crd` version `104.0.1+up1.9.0` with a custom `OperationTolerations` entry matching the taint; confirms it reaches `StatusDeployed` with the taint tolerated.

**Act:** Upgrades the chart to version `104.0.2+up1.9.0`, specifying the same custom toleration.

**Assert:**
- Checks the upgrade reaches `StatusDeployed`.
- Checks the upgraded operation's pod tolerations still include the custom taint.

## `TestUpgradeChartWithAutomaticTolerationOnTaintedCPNode`
**Arrange:**
- Adds a custom taint to a control plane node.
- Installs the chart at version `104.0.1+up1.9.0` with a custom toleration matching the taint; confirms it reaches `StatusDeployed` with the taint tolerated.

**Act:** Upgrades the chart to version `104.0.2+up1.9.0` with `AutomaticCPTolerations` enabled instead of an explicit toleration.

**Assert:**
- Checks the upgrade reaches `StatusDeployed`.
- Checks the upgraded operation's pod tolerations still include the custom taint.

## `TestUpgradeChartInstalledWithoutTolerationsUsingAutomaticTolerations`
**Arrange:**
- Installs the chart at version `104.0.1+up1.9.0` with no automatic CP tolerations and no custom tolerations; confirms it reaches `StatusDeployed` with only the default pod tolerations.
- Adds a custom taint to a control plane node.

**Act:** Upgrades the chart to version `104.0.2+up1.9.0` with `AutomaticCPTolerations` enabled.

**Assert:**
- Checks the upgrade reaches `StatusDeployed`.
- Checks the upgraded operation's pod tolerations now include the custom taint.

## `TestUninstallChartWithAutomaticTolerationOnTaintedCPNode`
**Arrange:**
- Adds a custom taint to a control plane node.
- Installs the chart with a custom toleration matching the taint; confirms it reaches `StatusDeployed` with the taint tolerated.

**Act:** Uninstalls the chart with `AutomaticCPTolerations` enabled.

**Assert:**
- Checks the chart reaches `StatusUninstalled`.
- Checks the uninstall operation's pod tolerations still include the custom taint.

## `TestUninstallChartWithCustomTolerationOnTaintedCPNode`
**Arrange:**
- Adds a custom taint to a control plane node.
- Installs the chart with a custom toleration matching the taint; confirms it reaches `StatusDeployed` with the taint tolerated.

**Act:** Uninstalls the chart, specifying the same custom toleration explicitly (no automatic CP tolerations).

**Assert:**
- Checks the chart reaches `StatusUninstalled`.
- Checks the uninstall operation's pod tolerations still include the custom taint.

## `TestInstallChartWithSkipSchemaValidation`
**Arrange:**
- None.

**Act:** Installs the chart with `SkipSchemaValidation` enabled.

**Assert:**
- Checks the chart reaches `StatusDeployed`.
- Checks the install operation's command includes `--skip-schema-validation=true`.

## `TestUpgradeChartWithSkipSchemaValidation`
**Arrange:**
- Installs the chart at version `104.0.2+up1.9.0` (without `SkipSchemaValidation`); confirms it reaches `StatusDeployed`.

**Act:** Upgrades the chart (same version) with `SkipSchemaValidation` enabled.

**Assert:**
- Checks the upgrade reaches `StatusDeployed`.
- Checks the upgrade operation's command includes `--skip-schema-validation=true`.
