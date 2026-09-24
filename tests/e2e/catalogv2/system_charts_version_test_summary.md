# `system_charts_version_test.go` Summary

Verifies that system charts (rancher-webhook and fleet) can be installed at specific versions and that the system deploys the latest available version when appropriate.

## `TestInstallWebhook`
Uninstalls rancher-webhook, sets the webhook version setting to 2.0.3+up0.3.3, watches for the App to be created, and verifies the installed release matches the specified version.
- Checks the app reaches StatusDeployed.
- Checks the installed Helm release version is 2.0.3+up0.3.3.

## `TestInstallFleet`
Uninstalls the fleet chart from cattle-fleet-system namespace, sets the fleet-min-version setting to 102.0.0+up0.6.0, watches for the App to be created, and verifies the deployed version matches the latest available.
- Checks the app reaches StatusDeployed.
- Checks the installed Helm release version equals the latest available version from the catalog.
