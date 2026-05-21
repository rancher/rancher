# Optimize Provisioning Test CI: Trim suites for PR runs, move heavy tests to nightly

## Problem

Every PR push to `rancher/rancher` triggers the full provisioning test suite — **7 parallel matrix jobs on 16-CPU runners** — regardless of what files changed. Combined with unreliable runner availability (wait times of 1h40min+ have been observed), CI is increasingly an anchor that slows down development velocity, especially as agentic coding assistance increases PR throughput.

### Current PR CI Timeline

```
Build (server+agent, x64+arm64)  ─── ~11 min ───┐
                                                  ├─> Integration tests ── ~30 min ──────────────┐
                                                  ├─> Provisioning Suite 1 ── ~32 min ────────┐  │
                                                  ├─> Provisioning Suite 2 ── ~31 min ──────┐ │  │
                                                  ├─> Provisioning Suite 3 ── ~27 min ────┐ │ │  │
                                                  ├─> Provisioning Suite 4 ── ~31 min ──┐ │ │ │  │
                                                  ├─> Provisioning Suite 5 ── ~35 min ┐ │ │ │ │  │
                                                  ├─> Provisioning Suite 6 ── ~8 min   │ │ │ │ │  │
                                                  └─> Provisioning Suite 7 ── ~7 min   │ │ │ │ │  │
                                                                                       └─└─└─└─└──└─> ~50-55 min total
```

- **Total wall-clock per PR: ~50-55 minutes**
- **Total provisioning compute: ~175 CPU-minutes** across 7 runners (each 16-CPU = **~2,800 vCPU-minutes**)
- Runs on every push, even for README changes, dashboard bumps, fleet updates, etc.

## Detailed Test Analysis

### Suite 1: k3s `General|Provisioning|Fleet` (~32 min, 17 tests)

| Test | Duration | What it tests | PR? |
|------|----------|---------------|-----|
| `Test_General_SystemAgentVersion` | <1s | Setting value matches env var | ✅ |
| `Test_General_WinsAgentVersion` | <1s | Setting value matches env var | ✅ |
| `Test_General_CSIProxyAgentVersion` | <1s | Setting value matches env var | ✅ |
| `Test_General_RKEMachinePool_Autoscaling_Field_Validation` | ~6s | Webhook validation of autoscaling fields (19 sub-cases) | ✅ |
| `Test_General_RKEMachinePool_Autoscaling_Update_Field_Validation` | ~4s | Webhook validation on updates (16 sub-cases) | ✅ |
| `Test_Fleet_Cluster` | ~52s | Fleet local cluster agent affinity, resource req propagation | ✅ |
| `Test_Fleet_ClusterBootstrap` | ~154s | Fleet cluster bootstrap + downstream cluster fleet registration | ✅ |
| `Test_Provisioning_Custom_OneNodeWithDelete` | ~109s | Custom cluster: 1 node all-roles, labels, delete (k3s-only) | ✅ |
| `Test_Provisioning_Custom_ThreeNode` | ~107s | Custom cluster: 3-node all-roles | 🌙 |
| `Test_Provisioning_Custom_UniqueRoles` | ~136s | Custom cluster: 5 nodes, unique roles (3 etcd, 1 cp, 1 worker) | 🌙 |
| `Test_Provisioning_Custom_ThreeNodeWithTaints` | ~139s | Custom cluster: 3 nodes with taints (k3s-only) | 🌙 |
| `Test_Provisioning_MP_SingleNodeAllRolesWithDelete` | ~138s | MP cluster: 1 node all-roles, nodeconfig verification, delete | ✅ |
| `Test_Provisioning_MP_MachineTemplateClonedAnnotations` | ~101s | MP: machine template annotation cloning (k3s-only) | 🌙 |
| `Test_Provisioning_MP_MachineSetDeletePolicyOldestSet` | ~139s | MP: 2 pools, MachineSet OldestDeletion policy (k3s-only) | 🌙 |
| `Test_Provisioning_MP_MultipleEtcdNodesScaledDownThenDelete` | ~235s | MP: 3-node, etcd scale-down, agent affinity, verify+delete | ✅ |
| `Test_Provisioning_MP_FiveNodesUniqueRolesWithDelete` | ~280s | MP: 5 nodes unique roles, create+delete (k3s-only) | 🌙 |
| `Test_Provisioning_MP_Drain` | ~161s | MP: drain with pre/post hooks, upgrade strategy | 🌙 |
| `Test_Provisioning_MP_DrainNoDelete` | ~86s | MP: drain-before-delete annotation checking | 🌙 |
| `Test_Provisioning_Single_Node_All_Roles_Drain` | ~234s | Single-node drain during upgrade, verifies drain behavior | 🌙 |
| `Test_Provisioning_MP_FourNodesServerAndWorkerRolesWithDelete` | ~206s | MP: 4-node (3 cp+etcd, 1 worker), create+delete (k3s-only) | 🌙 |

**Current duration: ~32 min** | **Estimated PR subset: ~8 min** (8 essential tests)

### Suite 2: rke2 `General|Provisioning|Fleet` (~31 min, same tests, rke2 distro)

Same tests as Suite 1, except tests with `DIST == "rke2"` skip (5 are k3s-only). The essential tests to keep are the same logical set.

**Current duration: ~31 min** | **Estimated PR subset: ~8 min** (fewer tests because k3s-only ones are skipped)

### Suite 3: k3s `Operation_.*` (~27 min, 9 tests)

This suite runs ALL Operation tests (SetA, SetB, SetC) for both custom and MP provisioned clusters against k3s.

| Test | Duration | What it tests | PR? |
|------|----------|---------------|-----|
| **Custom cluster operations** | | | |
| `Test_Operation_SetA_Custom_CertificateRotation` | ~193s | Certificate rotation on 3-node custom cluster | 🌙 |
| `Test_Operation_SetA_Custom_EtcdSnapshotCreationRestoreInPlace` | ~218s | Etcd snapshot create + in-place restore, custom 2-node | ✅ |
| `Test_Operation_SetA_Custom_EncryptionKeyRotation` | skipped | RKE2-only, skipped on k3s | — |
| `Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewNode` | ~509s | Snapshot + delete etcd node + new node + restore from file | ✅ 🔥 |
| `Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewCombinedNode` | ~309s | Same as above but combined cp+etcd node | 🌙 |
| **Machine provisioned operations** | | | |
| `Test_Operation_SetA_MP_CertificateRotation` | ~101s | Certificate rotation, 1-node MP cluster | ✅ |
| `Test_Operation_SetA_MP_EtcdSnapshotCreationRestoreInPlace` | ~229s | Etcd snapshot create + in-place restore, 2-node MP | 🌙 |
| `Test_Operation_SetA_MP_EncryptionKeyRotation` | skipped | RKE2-only, skipped on k3s | — |
| `Test_Operation_SetB_MP_EtcdSnapshotOperationsOnNewNode` | ~472s | S3 snapshot, scale etcd to 0, scale to 1, restore | 🌙 |
| `Test_Operation_SetB_MP_EtcdSnapshotOperationsWithThreeEtcdNodesOnNewNode` | ~352s | 5-node with S3, scale down all cp+etcd, restore | 🌙 |
| `Test_Operation_SetC_MP_DataDirectories` | ~106s | Custom data directories for system-agent/provisioning/k8s | ✅ |

> 🔥 `Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewNode` is the **lightest disaster recovery test** (~509s / ~8.5 min on k3s) that covers the full DR flow: snapshot → delete etcd node → create new node → restore from snapshot file. Including it ensures PR runs catch regressions in the critical etcd recovery path without adding the heavier S3-based or multi-etcd-node variants.

**Current duration: ~27 min** | **Estimated PR subset: ~16 min** (4 essential tests including 1 DR test)

### Suite 4: rke2 `Operation_SetA_.*` (~31 min, 4 tests)

| Test | Duration | What it tests | PR? |
|------|----------|---------------|-----|
| `Test_Operation_SetA_Custom_CertificateRotation` | ~200s | Cert rotation, custom 3-node | 🌙 |
| `Test_Operation_SetA_Custom_EncryptionKeyRotation` | ~300s | Encryption key rotation, custom 3-node (RKE2-only) | ✅ |
| `Test_Operation_SetA_Custom_EtcdSnapshotCreationRestoreInPlace` | ~250s | Snapshot + in-place restore, custom 2-node | 🌙 |
| `Test_Operation_SetA_MP_CertificateRotation` | ~120s | Cert rotation, 1-node MP | 🌙 |
| `Test_Operation_SetA_MP_EncryptionKeyRotation` | ~300s | Encryption key rotation, 1-node MP (RKE2-only) | ✅ |
| `Test_Operation_SetA_MP_EtcdSnapshotCreationRestoreInPlace` | ~547s | Snapshot + in-place restore, 2-node MP | 🌙 |

**Current duration: ~31 min** | **Estimated PR subset: ~10 min** (2 essential RKE2-specific tests)

### Suite 5: rke2 `Operation_SetB_.*` (~35 min, 3 tests)

| Test | Duration | What it tests | PR? |
|------|----------|---------------|-----|
| `Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewNode` | ~550s | Snapshot, replace etcd node, restore from file | ✅ 🔥 |
| `Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewCombinedNode` | ~400s | Same, combined cp+etcd node | 🌙 |
| `Test_Operation_SetB_MP_EtcdSnapshotOperationsOnNewNode` | ~893s | S3 snapshot, scale etcd to 0, scale to 1, restore | 🌙 |
| `Test_Operation_SetB_MP_EtcdSnapshotOperationsWithThreeEtcdNodesOnNewNode` | — | 5-node S3, scale down, restore (may not match SetB regex) | 🌙 |

> 🔥 `Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewNode` (~550s / ~9 min on rke2) provides DR coverage for the rke2 distro. This is the same logical test as in Suite 3 but against rke2, ensuring both distros have their DR path validated on every PR.

**Current duration: ~35 min** | **Estimated PR subset: ~9 min** (1 essential DR test)

### Suite 6: k3s `PreBootstrap_.*` (~7 min, 1 test with 2 subtests)

| Test | Duration | What it tests | PR? |
|------|----------|---------------|-----|
| `Test_PreBootstrap_Provisioning_Flow/Generic_Secret_Sync` | ~3 min | Pre-bootstrap secret sync with `{{clusterId}}` templating | ✅ |
| `Test_PreBootstrap_Provisioning_Flow/ACE` | ~3 min | Pre-bootstrap with AuthClusterEndpoint enabled | ✅ |

**Current duration: ~7 min** | **Estimated PR subset: ~7 min** (keep all — already fast and tests a feature-flagged flow)

### Suite 7: rke2 `PreBootstrap_.*` (~8 min, 1 test with 2 subtests)

Same as Suite 6, against rke2.

**Current duration: ~8 min** | **Estimated PR subset: ~8 min** (keep all)

## Proposal: Path-Based Test Level Selection

Not all PRs need the same level of test coverage. We propose a **three-tier system** based on which files the PR modifies:

### Tier 1: Skip provisioning tests entirely

If a PR **only** modifies files matching these patterns, provisioning tests are skipped completely (unit tests and linting still run):

| Pattern | Rationale |
|---------|-----------|
| `docs/**` | Documentation only |
| `**/*.md` | Markdown files (README, CONTRIBUTING, etc.) |
| `*.md` | Root-level markdown |
| `code-of-conduct.md`, `CONTRIBUTING.md`, `keybase.md` | Repo meta files |
| `LICENSE` | License file |
| `.gitignore`, `.gitattributes`, `.dockerignore` | Git/Docker config |
| `tests/validation/**` | Validation tests (already in paths-ignore) |
| `tests/v2/codecoverage/**` | Code coverage config (already in paths-ignore) |
| `updatecli/**` | Updatecli dependency update configs |
| `dev-scripts/**` | Local development scripts |
| `hack/**` | Dev tooling and scripts |
| `.github/workflows/publish-*.yml` | Publish/release workflows (don't affect build) |
| `.github/workflows/stale.yml` | Stale issue/PR bot |
| `.github/ISSUE_TEMPLATE/**` | Issue templates |
| `.github/CODEOWNERS` | Code ownership config |
| `chart/Chart.yaml` (version-only bumps) | Helm chart metadata |

### Tier 2: Run FULL provisioning tests

If a PR modifies **any** file matching these patterns, the **full** (current) provisioning test suite runs — all 7 suites with all tests:

| Pattern | Rationale |
|---------|-----------|
| `pkg/capr/**` | Core CAPR provisioning logic (planner, bootstrap, etcd management, machine provisioning, drain, RKE control plane) |
| `pkg/provisioningv2/**` | Provisioning v2 subsystem (kubeconfig, image resolution, system info, prebootstrap) |
| `pkg/controllers/provisioningv2/**` | Provisioning v2 controllers (cluster, fleet cluster, managed charts, secrets, provisioning log) |
| `pkg/controllers/capr/**` | CAPR controllers (autoscaler, etcd mgmt, machine drain, plan secrets, dynamic schema) |
| `pkg/fleet/**` | Fleet integration (cluster registration, agent configuration) |
| `pkg/plan/**` | Plan system (node plans, probes, state management) |
| `pkg/taints/**` | Node taint utilities |
| `pkg/rkecerts/**` | RKE certificate expiration/rotation logic |
| `pkg/node/**` | Node management utilities |
| `pkg/cluster/**` | Cluster agent customization, private registry config |
| `pkg/controllers/dashboard/fleetcharts/**` | Fleet chart management |
| `pkg/controllers/management/node*` | Management-level node controllers |
| `pkg/settings/**` | Settings (system-agent versions, etc.) |
| `tests/v2prov/**` | The provisioning test framework itself |
| `package/Dockerfile*` | Changes to container images affect provisioning |
| `scripts/provisioning-tests` | The provisioning test runner script |
| `.github/workflows/provisioning-tests.yml` | Provisioning CI workflow definition |

### Tier 3: Run trimmed (default) provisioning tests

For all other PRs — the majority of day-to-day changes — run the **trimmed** suite described in the next section. This covers:
- Changes to `pkg/api/**`, `pkg/auth/**`, `pkg/rbac/**`, `pkg/data/**`, etc.
- Changes to `go.mod`, `go.sum` (dependency updates)
- Changes to integration tests, Python tests, etc.
- Changes to build scripts, Makefiles, etc.
- General `pkg/controllers/management*/**` changes (non-provisioning controllers)

### Implementation Approach

The path-based logic can be implemented as a **reusable job** that runs before the provisioning tests and sets an output variable:

```yaml
  determine-test-level:
    runs-on: ubuntu-latest
    outputs:
      test_level: ${{ steps.check.outputs.level }}
    steps:
      - uses: actions/checkout@v4
      - id: check
        run: |
          # Get changed files
          CHANGED=$(gh pr diff ${{ github.event.pull_request.number }} --name-only)

          # Check for Tier 2 (full tests) — provisioning-critical paths
          if echo "$CHANGED" | grep -qE '^(pkg/capr/|pkg/provisioningv2/|pkg/controllers/provisioningv2/|pkg/controllers/capr/|pkg/fleet/|pkg/plan/|pkg/taints/|pkg/rkecerts/|pkg/node/|pkg/cluster/|pkg/settings/|tests/v2prov/|package/Dockerfile|scripts/provisioning-tests|\.github/workflows/provisioning-tests\.yml)'; then
            echo "level=full" >> $GITHUB_OUTPUT
            exit 0
          fi

          # Check for Tier 1 (skip tests) — docs/meta only
          NON_SKIP=$(echo "$CHANGED" | grep -vE '^(docs/|.*\.md$|LICENSE|\.git(ignore|attributes)|\.dockerignore|tests/validation/|tests/v2/codecoverage/|updatecli/|dev-scripts/|hack/|\.github/(ISSUE_TEMPLATE|CODEOWNERS|workflows/(publish-|stale)))')
          if [ -z "$NON_SKIP" ]; then
            echo "level=skip" >> $GITHUB_OUTPUT
            exit 0
          fi

          # Default: Tier 3 (trimmed tests)
          echo "level=trimmed" >> $GITHUB_OUTPUT
```

The provisioning test workflow then uses this output:

```yaml
  provisioning_tests:
    needs: [determine-test-level]
    if: needs.determine-test-level.outputs.test_level != 'skip'
    strategy:
      matrix:
        include: ${{ needs.determine-test-level.outputs.test_level == 'full' && <full-matrix> || <trimmed-matrix> }}
```

## Proposal: Trimmed PR Suites (Tier 3 / Default)

Instead of going "all or nothing" on each suite, we keep all 7 matrix jobs but trim each to only run essential tests. This maintains **wide distro × test-type coverage** while dramatically reducing execution time. One disaster recovery test is included per distro to catch regressions in the critical etcd recovery path.

### Proposed PR Regex Configuration

```yaml
strategy:
  fail-fast: false
  matrix:
    include:
    # Suite 1: k3s smoke — basic provisioning + fleet + general validation
    - V2PROV_TEST_DIST: "k3s"
      V2PROV_TEST_RUN_REGEX: "^Test_(General|Fleet|Provisioning_Custom_OneNodeWithDelete|Provisioning_MP_SingleNodeAllRolesWithDelete|Provisioning_MP_MultipleEtcdNodesScaledDownThenDelete)_.*$"

    # Suite 2: rke2 smoke — same core tests on rke2
    - V2PROV_TEST_DIST: "rke2"
      V2PROV_TEST_RUN_REGEX: "^Test_(General|Fleet|Provisioning_Custom_OneNodeWithDelete|Provisioning_MP_SingleNodeAllRolesWithDelete|Provisioning_MP_MultipleEtcdNodesScaledDownThenDelete)_.*$"

    # Suite 3: k3s operations — snapshot + cert rotation + data dirs + 1 DR test
    - V2PROV_TEST_DIST: "k3s"
      V2PROV_TEST_RUN_REGEX: "^Test_Operation_(SetA_Custom_EtcdSnapshotCreationRestoreInPlace|SetA_MP_CertificateRotation|SetB_Custom_EtcdSnapshotOperationsOnNewNode|SetC_MP_DataDirectories)$"

    # Suite 4: rke2 operations — encryption key rotation (RKE2-only)
    - V2PROV_TEST_DIST: "rke2"
      V2PROV_TEST_RUN_REGEX: "^Test_Operation_SetA_(Custom_EncryptionKeyRotation|MP_EncryptionKeyRotation)$"

    # Suite 5: rke2 DR smoke — 1 disaster recovery test on rke2
    - V2PROV_TEST_DIST: "rke2"
      V2PROV_TEST_RUN_REGEX: "^Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewNode$"

    # Suite 6: k3s PreBootstrap (already fast, keep as-is)
    - V2PROV_TEST_DIST: "k3s"
      V2PROV_TEST_RUN_REGEX: "^Test_PreBootstrap_.*$"
      CATTLE_FEATURES: "provisioningprebootstrap=true"

    # Suite 7: rke2 PreBootstrap (already fast, keep as-is)
    - V2PROV_TEST_DIST: "rke2"
      V2PROV_TEST_RUN_REGEX: "^Test_PreBootstrap_.*$"
      CATTLE_FEATURES: "provisioningprebootstrap=true"
```

### Expected Impact

| Metric | Current (7 suites) | Proposed — trimmed (7 suites) | Proposed — skip (0 suites) | 
|--------|-------------------|-------------------------------|---------------------------|
| Longest suite wall-clock | ~35-40 min | ~16 min | 0 min |
| Total provisioning compute | ~175 CPU-min | ~65 CPU-min | 0 min |
| Number of parallel runners | 7 × 16-CPU | 7 × 16-CPU | 0 |
| Total vCPU-minutes | ~2,800 | ~1,040 | 0 |
| **Savings vs. current** | — | **~63%** | **100%** |

With path-based skipping (Tier 1), PRs touching only docs/meta files save **all** provisioning compute.  
With provisioning-critical path detection (Tier 2), risky PRs still get the **full** suite.  
Default PRs (Tier 3) get **wide but trimmed** coverage with ~63% compute savings.

### What Stays Covered on PRs (Tier 3)

- ✅ **General settings validation** (system-agent, wins, CSI proxy versions)
- ✅ **Autoscaler webhook validation** (create + update, 35 sub-cases)
- ✅ **Fleet integration** (local cluster bootstrap, downstream cluster registration, agent customization)
- ✅ **Custom cluster basic provisioning** (1-node with delete, on both k3s and rke2)
- ✅ **Machine provisioned basic provisioning** (1-node with delete + etcd scale-down, on both k3s and rke2)
- ✅ **Etcd snapshot create + in-place restore** (custom cluster, k3s)
- ✅ **Certificate rotation** (MP cluster, k3s)
- ✅ **Encryption key rotation** (custom + MP, rke2-only)
- ✅ **Custom data directories** (k3s)
- ✅ **Disaster recovery** — etcd snapshot → delete etcd node → new node → restore (both k3s and rke2)
- ✅ **PreBootstrap provisioning flow** (secret sync + ACE, both distros)

### What Moves to Nightly Only

- 🌙 Multi-node provisioning variations (3-node, 5-node, unique roles)
- 🌙 Taint handling in multi-node clusters
- 🌙 Machine template annotation cloning
- 🌙 MachineSet delete policy validation
- 🌙 Drain hooks (pre/post hooks, drain-before-delete, single-node drain)
- 🌙 Advanced etcd disaster recovery variants (combined cp+etcd node restore, S3 snapshots)
- 🌙 Multi-etcd-node S3 snapshot with scale-down and restore
- 🌙 Duplicate certificate rotation runs (keep 1, nightly the rest)

## Nightly Run Infrastructure

1. Create a scheduled workflow (e.g., nightly at 2:00 AM UTC) running the **full** current test matrix
2. Report results via the existing `publish-provisioning-test-results.yaml` workflow
3. Send notifications to the Hostbusters team for monitoring
4. Gate releases on nightly results being green (within acceptable flake threshold)

## Integration Tests Cross-Reference (Separate Enhancement)

Integration tests currently run in a single job (~30 min) and are close to the critical path:
- Go integration tests (`-p 1`, sequential): ~18 packages covering catalogv2, rbac, projects, steveapi, users, serviceaccount, clusters, authconfigs
- Python tox tests: 22 test files (parallel + nonparallel)

These are **not yet the bottleneck** since the longest provisioning suite (~35-40 min) exceeds them today. However, once provisioning suites are trimmed to ~16 min, integration tests (~30 min) will become the new critical path. A separate enhancement should address:
- Parallelizing Go integration test packages (currently `-p 1`)
- Splitting Python tox tests into a separate workflow job
- Path-based skipping for unrelated integration test packages

## Context

From team discussion (2026-05-19/20):
> "As we move faster with agentic coding assistance, CI (integration/provisioning/e2e tests) is increasingly becoming an anchor holding us all back." — @pmatseykanets
>
> "To cull majority of provisioning tests from the PR run and move the rest to nightly run with reports sent to Hostbusters to babysit." — @nasovich

## Data Sources

Per-test timing extracted from CI logs of recent successful PR runs:
- [Run 26191448223](https://github.com/rancher/rancher/actions/runs/26191448223) — "Add SameSite attribute to Cookies" — 50 min total
- [Run 26191842095](https://github.com/rancher/rancher/actions/runs/26191842095) — "Add SameSite attribute to Cookies" — 55 min total
- [Run 26193359766](https://github.com/rancher/rancher/actions/runs/26193359766) — "Add SameSite attribute to Cookies" — 54 min total
