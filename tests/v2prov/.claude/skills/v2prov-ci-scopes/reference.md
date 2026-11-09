# v2prov CI scopes reference

Background for placing tests in `.github/scripts/provisioning-test-scopes.yaml`. The header comment in that file and in `.github/scripts/resolve-provisioning-matrix.sh` are the source of truth for the config format. This file covers the conventions and constraints that aren't written there.

## How a PR picks its tests

1. `resolve-provisioning-matrix.sh` diffs the PR against its base.
2. A changed file that matches a `full` entry selects every scope under `scopes`.
3. Otherwise each scope whose `package-paths` (or `file-paths`, with optional substring rules) match a changed file is selected.
4. The selected scopes' `tests` rows become the GHA matrix, one job per row per dist.

Row keys are the environment variable names the workflow reads (`V2PROV_TEST_DIST`, `V2PROV_TEST_RUN_REGEX`, `CATTLE_FEATURES`, `V2PROV_TEST_CPUS`, `V2PROV_TEST_PARALLELISM`). The resolver passes every key on a row through to the matrix unchanged, then fills in defaults for dist (both), cpus (16) and parallelism (1). The scope's `meta.V2PROV_TEST_TIMEOUT` is added as well. Keys this repo doesn't read are still passed through. For example, kdm rows set `V2PROV_TEST_RUN_ON_RELEASE`, which is for downstream consumers. Keep such keys on any row you edit or split, and copy them onto new rows in the same scope. Every key also shows up in the GHA job name, so don't add keys just for documentation.

`explicit` scopes (`nightly`, `kdm`) are never picked by a diff, not even with `ALL=1`. They only run when requested by name. So a test can sit in a PR scope and in nightly at the same time without running twice.

## Scopes

| Scope | Purpose | Test dirs |
|---|---|---|
| `basic` | general, custom and machine provisioning, fleet, agent | `general/`, `custom/`, `machineprovisioning/`, `fleet/`, `autoscaler/` |
| `operations` | day-2 ops (cert rotation, encryption key rotation, etcd snapshot, drain) on custom and MP clusters | `custom/`, `machineprovisioning/` |
| `imported` | imported cluster lifecycle and day-2 ops | `imported/` |
| `prebootstrap` | prebootstrap secrets flow, on k3s (rke2 is covered by basic's `General` row) | `prebootstrap/` |
| `full` (paths only) | core code where any change can break all provisioning: planner, plan, rkecontrolplane, rkecluster, the v2prov framework, the runner | n/a |
| `skip` (paths only) | provisioning-related code with no v2prov coverage, listed so nobody adds it to a scope by accident | n/a |
| `nightly` (explicit) | everything that should run on merged code, once per day per listed branch | all |
| `kdm` (explicit) | subset run when validating KDM data changes (new k8s versions) | k8s-version-sensitive tests |

A package can appear in more than one scope. The file marks this with a trailing `# also <scope>` comment; keep that convention.

## Naming and tiers

The test name decides which row picks it up. Prefer giving a test the right name over widening a regex to catch it.

| Prefix | Where it runs |
|---|---|
| `Test_General_*` | basic (rke2 only), nightly (rke2 only). `Test_General_PreBootstrap_*` also runs on k3s via the `prebootstrap` scope and a k3s nightly row |
| `Test_Fleet_*` | basic (rke2 only), nightly (rke2 only) |
| `Test_Provisioning_SetA_*`, `Test_Provisioning_Custom_SetA_*` | basic (both dists), nightly, kdm |
| `Test_Provisioning_SetB_*`, `Test_Provisioning_Custom_SetB_*` | nightly and kdm only |
| `Test_Operation_SetA_*` | operations (both), nightly, kdm |
| `Test_Operation_SetB_*` | operations (k3s all; rke2 split into a parallel `EtcdSnapshotOperationsOnNew` row and one named test), nightly, kdm |
| `Test_Operation_SetC_*` | operations (k3s only), nightly (k3s only), kdm |
| `Test_Imported_Operation_SetD_*` | imported, but only the tests named explicitly in its rows; nightly lists most of the rest by name |
| `Test_Imported_Operation_SetE_*` | nightly only (heavy multi-node restore work) |

For provisioning tests, SetA is the cheap PR tier and SetB is nightly-only. For operations, the letters are shard buckets, not tiers. In imported, SetD rows enumerate names, so adding a SetD test always means editing a row.

`MP` in a name means machine provisioning (pod-backed machines via `nodeconfig`). `Custom` means custom clusters (`systemdnode` pods joined with the registration command).

## Should a test run on PRs?

Put it in a PR scope only if all of these hold:

- It guards code that changes often enough that a regression should block a merge.
- No PR-tier test already exercises the same path. If one does, the new test goes nightly-only.
- It doesn't depend on env that PR CI doesn't provide (for example `V2PROV_TEST_CAPRKE2`, a dual-stack host).

Otherwise it goes in nightly only. Every test that runs on PRs must also run in nightly, for each dist it runs on in PRs.

kdm is for behavior that changes with the Kubernetes version: etcd snapshot and restore, cert rotation, encryption key rotation, basic provisioning. Default to not adding a test to kdm unless the author says it covers something like that.

## Cost model

- Every cluster node is a pod on one host cluster, on one runner. Cost is roughly node count times wall time.
- Node count by helper:
  - Custom: one `systemdnode.New(...)` per node.
  - MP: `RKEConfig.MachinePools[].Quantity` (defaults to `nodeconfig.NewPodConfig`).
  - Imported: the `[]cluster.ImportedNodePool{...Quantity}` passed to `setUpImportedCluster`.
- `go test -p 3` (`V2PROV_PACKAGE_PARALLELISM`) already runs up to three packages at once. A row matching tests in `custom/` and `machineprovisioning/` runs those concurrently whatever `V2PROV_TEST_PARALLELISM` says.
- `V2PROV_TEST_PARALLELISM` becomes `go test -parallel`. It only affects tests that call `t.Parallel()`. Setting it on a row where no test does is a no-op.
- `V2PROV_TEST_CPUS` only picks the runner label (`runner=<cpus>cpu-linux-x64`). Default is 16. The imported rows that run 1-node tests one at a time use 8. Larger multi-node or parallel rows need 16.
- `-failfast` is on: one failure stops the rest of that shard. Smaller shards fail more precisely.

When nightly timing data isn't available for the target branch, estimate with node count and the closest existing test, and say so to the author.

## Hard constraints

- **Regex characters.** `scripts/provisioning-tests` rejects any regex outside `^[][A-Za-z0-9_()|.^$*?]+$`. Character classes and `?` are allowed. No `+`, `{}`, `-`, spaces or `\`. Prefer alternation of full names when it reads more clearly.
- **No lookahead.** Go uses RE2. To carve a test out of a `.*` row, enumerate the rest by name or rename tests so the prefixes separate.
- **No overlap.** Rows in one scope must match disjoint tests for the same dist. Across PR scopes this also matters, since several scopes can fire on one PR and the resolver doesn't de-duplicate.
- **Timeouts.** PR rows get the 60m `go test` default. Only explicit scopes can set `meta.V2PROV_TEST_TIMEOUT`. The GHA job is capped at 360m including setup.
- **Matrix cap.** `provisioning-tests.yml` fails if the matrix has more than 25 entries. Count expanded rows: a row with no `V2PROV_TEST_DIST` counts twice. A `full` trigger runs the union of all PR scopes, so their combined row count must also stay at or under 25.
- **Artifact names.** Each job uploads `Test Results <dist> <suite>`, where `<suite>` is the regex with `Test` removed and `[]()|_^.*$?` stripped. Two rows in the same run with the same dist and suite overwrite each other.
- **New scopes.** A new scope needs a `'^trigger-<scope>-ci$'` file-paths entry and must be added to the scope choices in `.github/workflows/ci-testing-create-pr.yml`.

## Balancing

- Target: a shard's `Run tests` step should stay under about 70% of its timeout (about 40m for PR rows).
- Add to the lightest compatible row first: same scope, same dist set, same `CATTLE_FEATURES`, compatible `V2PROV_TEST_CPUS`.
- Split a row only when adding the test would push it past the target. Split by adding a new row that is mutually exclusive with the old one:
  - Prefer a name-based split (move tests to a new Set letter or a distinct name fragment).
  - Otherwise enumerate names in an alternation, as the imported rows do.
- A test that calls `t.Parallel()` can share a row with other parallel tests if `V2PROV_TEST_PARALLELISM` is raised. That only helps if the runner has the CPU for the extra nodes.
- When splitting a PR row, check whether the matching nightly row needs the same split. Nightly rows run on a 200m timeout, so they can usually absorb more.

## Known drift (as of 2026-11)

`check-coverage.sh` reports these. They are known and accepted for now. Mention them to the author once per session, don't offer to fix them, and don't count them against a proposed change:

- **Orphans.** `Test_Autoscaling_Provisioning`, the `ImportedServerJoinOutsideCertificateSANs` and `StaleAddressOutsideLeaderCertificate` SetD tests, `...LocalHAIPv6`, and the six `CAPRKE2Docker*` SetE tests match no row. The CAPRKE2 and IPv6 tests are deliberately local-only.
- **No-op parallelism.** Some operations rows set `V2PROV_TEST_PARALLELISM`, but no Operation test calls `t.Parallel()`.
- **Nightly is near the cap.** 23 of 25 rows.

Anything else the script reports that isn't caused by the author's change is new drift. Offer a fix, and apply it only if the author agrees.
