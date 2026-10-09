# `system_charts_version_test.go` Summary

Verifies that Rancher-managed system charts (rancher-webhook and fleet) deploy at the version dictated by their version settings, deploying the latest available version when the configured constraint allows it.

## `TestInstallWebhook`
**Arrange:**
- Uninstalls the existing `rancher-webhook` release.

**Act:** Sets the `rancher-webhook-version` setting to an exact version (`2.0.3+up0.3.3`).

**Assert:**
- Checks the `rancher-webhook` App is (re)created.
- Checks the installed Helm release version matches the exact configured version.

## `TestInstallFleet`
**Arrange:**
- Uninstalls the existing `fleet` release from the `cattle-fleet-system` namespace.

**Act:** Sets the `fleet-min-version` setting to a version below the latest available (`102.0.0+up0.6.0`).

**Assert:**
- Checks the `fleet` App is (re)created.
- Checks the deployed Helm release version equals the latest version available from the catalog (Rancher deploys latest when the configured minimum is below it, not the minimum itself).
