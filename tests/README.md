# Rancher Test Framework

The Rancher Test Framework provides tools for writing integrations and validation tests.  The framework manages interactions with the external services being tested and aids in cleaning up resources after a test is completed.  The framework is organized into three disciplines: framework, clients, and extensions.  The framework consists of a few core libraries used to make it easy to write homologous tests. See [extensions](#extensions) and [clients](#clients) for more details.

## Requirements

---

#### Integration
To run rancher integration tests you will need:
- a running instance of rancher with an accessible url
- a rancher access token for the admin user
- [golang 1.16](https://go.dev/doc/install)
- [k3d](https://k3d.io/v5.1.0/)

#### Validation
Validation tests have the same requirements as integration tests however different suites may need credentials for cloud providers.  Check the configs for your test suite for details.

## Concepts

---

### Integration vs Validation

TODO

### Extensions

Extensions are functions that complete common operations used by tests.  Extensions should not require much configuration or support multiple behaviors.  Extensions should be simple enough that they can be used in many tests without needing to be tested themselves.

### Clients

TODO

### Sessions

Sessions are used to track resources created by tests.  A session allows cleanup functions to be registered while it is open.  Once a session is closed the cleanup functions will be called latest to oldest.  Sessions should be closed after a set of tests that use the same resources is completed.  This eliminates the need for each test to create and tear down its own resources allowing for more efficient reuse of some resources.  When pared with a client sessions are a powerful tool that can track and cleanup any resource a tests creates with no additional work from the developer.

### Configuration

Configuration is loaded from the yaml or json file described in `CATTLE_TEST_CONFIG`.  Configuration objects are loaded from their associated key in the configuration file.  Default values can also be set on configuration objects.


## How to Write Tests

Tests should be created under `tests/v2/integration` or `tests/v2/validation`, see [Integration vs Validation](#integration-vs-validation) for details. Tests can be grouped into files and packages however the developer sees fit.  When grouping tests a developer should make sure the test will be easy to find for the next developer and that it is grouped with other tests that would be run at the same time.  Within the files tests should be grouped into [Suites](https://pkg.go.dev/github.com/stretchr/testify/suite).  A suite should share clients and a session across all it's tests.  A Suite's tests should all be testing the same functionality and reuse suite resources.  For example if we were writing tests that ensure project roles have appropriate access to project resources, a suite could contain all tests for every role and share a project across all tests. This will save time because each test will not need to create a project.  Tests should always use the framework clients when possible, this will ensure that any resources created by tests will be cleaned up and not interfere with other tests.  A test is responsible for cleaning up any resource it creates fully before the next test is run.  See [Sessions](#sessions) for how to track resources in tests. 
