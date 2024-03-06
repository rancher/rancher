# Rancher Test Framework

The Rancher Test Framework provides tools for writing integrations and validation tests.  The framework manages interactions with the external services being tested and aids in cleaning up resources after a test is completed.  The framework is organized into three disciplines: framework, clients, and extensions.  The framework consists of a few core libraries used to make it easy to write homologous tests. See [extensions](#extensions) and [clients](#clients) for more details.

## Requirements

---

#### Integration
To run rancher integration tests you will need:
- a running instance of rancher with an accessible url
- a rancher access token for the admin user
- [golang 1.22](https://go.dev/doc/install)
- [k3d](https://k3d.io/v5.1.0/)

#### Validation
Validation tests have the same requirements as integration tests however different suites may need credentials for cloud providers.  Check the configs for your test suite for details.

## Concepts

---

### Integration vs Validation

Integration tests - Don't require any external configuration or access to any other external services. These should also be short running as they will run on every PR within CI. Integration tests do not need access keys to cloud providers to run.

Validation tests - Require access to external services, and needs to a config file to run them.

## Framework

Shepherd is our testing framework and is located https://github.com/rancher/shepherd. For more info please visit the repo.


## How to Write Tests

---

Tests should be created under `tests/v2/integration` or `tests/v2/validation`, see [Integration vs Validation](#integration-vs-validation) for details. Tests can be grouped into files and packages however the developer sees fit.  When grouping tests a developer should make sure the test will be easy to find for the next developer and that it is grouped with other tests that would be run at the same time.  Within the files tests should be grouped into [Suites](https://pkg.go.dev/github.com/stretchr/testify/suite).  A suite should share clients and a session across all it's tests.  A Suite's tests should all be testing the same functionality and reuse suite resources.  For example if we were writing tests that ensure project roles have appropriate access to project resources, a suite could contain all tests for every role and share a project across all tests. This will save time because each test will not need to create a project.  Tests should always use the framework clients when possible, this will ensure that any resources created by tests will be cleaned up and not interfere with other tests.  A test is responsible for cleaning up any resource it creates fully before the next test is run.  See [Sessions](#sessions) for how to track resources in tests.
