# Rancher Testing Scope Audit

This document is the **research dump** for the testing audit: the suite-by-suite inventory
(§1), the overlap/redundancy findings (§2), related in-flight work (§3), and the flakiness
analysis (§4). It is descriptive — it records *what exists and how it runs today*.

The **forward-looking options** that came out of this research — path-based test tiering, the
Norman→wrangler API migration, and scheduled test runs — live in the companion
[Testing Suggestions](./testing-suggestions.md) document.

## 1. Current State Audit

**At a glance.** The table below summarizes every suite audited in this section; details follow
underneath.

| Suite | What it covers | Where it runs in CI | Requirements | Staleness |
|---|---|---|---|---|
| `tests/kev2` (AKS/EKS/GKE unit) | Hosted-cluster operator-controller reconcile logic (fully mocked) | Every PR **and** push (`unit-test.yml`, `./pkg/...`) | Go toolchain only | Runs always, but **no new coverage in ~2 yr**; README stale |
| `tests/pkg/serviceaccounttoken` | SA token-secret creation race / optimistic locking | **No CI path found — likely a gap** | envtest (apiserver + etcd) | Single test since 2024; upkeep only |
| `tests/testdata` (fixtures) | Shared Helm/OCI + UI-plugin fixtures for catalog tests | Wherever its consumers run (unit + integration) | None (static files) | **Actively maintained** |
| `tests/controllers` (envtest) | Per-controller reconcile vs. a real apiserver (feature, authconfig, globalroles, oidc) | Every PR **and** `release/*` push (`controller-test.yml`, `ubuntu-latest`) | envtest (apiserver + etcd) | Slow but alive; README Norman section `TODO`; Go 1.22 drift |
| `tests/v2/integration` (API e2e) | Rancher control-plane over its APIs (rbac, catalog, projects, steve, users, …) | **PR-only** (`integration-tests.yml`, EIO 16-CPU) | Real Rancher + k3d downstream cluster | **Actively growing** (Python→Go migration) |
| `tests/v2prov` (provisioning e2e) | v2prov + CAPR: builds real clusters, runs day-2 ops | **PR-only**, 9-leg sharded matrix (EIO 16-CPU) | Full stack; nested `systemd-node` clusters | **Most active suite** in the audit |

### `tests/kev2` — KEv2 (AKS/EKS/GKE) hosted-cluster unit tests

**What it is.** `tests/kev2/README.md` is documentation-only — there is no test code in
`tests/kev2` itself. It describes a set of Go unit tests that actually live alongside the
source they test, under `pkg/controllers/management/{aks,eks,gke}/`, because Go's testing
package requires test files to share a package with the code under test.

**What's tested.** Each provider package (`aks`, `eks`, `gke`) has an `OperatorController`
handler (`{provider}_cluster_handler.go`) responsible for reconciling a Rancher `Cluster`
against a hosted-Kubernetes operator CR (`AKSClusterConfig`, `EKSClusterConfig`,
`GKEClusterConfig`). The tests drive `on{Provider}ClusterChange`/`onClusterChange` through
its phases — nil cluster, nil provider config, default, create, active, update — and assert
the resulting cluster/config objects and calls made against mocked dependencies
(`ClusterClient`, `DynamicClient`, secrets, dialer, discovery, etc.).

**Files used, per provider directory:**
- `{provider}_cluster_handler_test.go` — the actual `Test_onClusterChange_*` test cases.
- `{provider}_cluster_handler_mockc_test.go` — hand-maintained mocks/harness (originally
  scaffolded by `mockcompose`, now largely manual — `mockcompose` is not in `go.mod`) that
  builds a `mock{Provider}OperatorController` wired to `go.uber.org/mock/gomock` fakes and
  `testify/mock`.
- `{provider}_cluster_handler_mockc_interface_test.go` — mock implementations of interfaces
  (`dynamic.NamespaceableResourceInterface`, dialer factory, discovery, etc.) used by the
  controller.
- `test/*.yaml` and `test/*.json` — fixture data (cluster objects and provider CC objects in
  default/create/active/update states) loaded via `//go:embed test/*`.

**Test infrastructure needed.** None beyond the Go toolchain — these are pure unit tests
with no cluster, network, or external service dependencies; everything is mocked or loaded
from embedded fixture files. The README's listed requirement ("golang 1.17") is stale; the
repo currently builds with the version pinned in `go.mod` (Go 1.24+ toolchain via
`GOTOOLCHAIN`).

**Where they're referenced / run:**
- Not individually referenced by name anywhere — they're swept up by the blanket
  `go test -cover -tags=test ./pkg/...` invocation.
- CI: `.github/workflows/unit-test.yml` runs exactly that command, and is called by both
  `pull-request.yml` and `push.yml` (job `unit-tests`), so these tests run on every PR and
  push.
- `scripts/test` (used by Drone/legacy pipelines) also runs `go test -cover -tags=test
  ./pkg/...` before moving on to integration tests.
- No `go:generate` directives exist in these packages (despite the mockcompose origin) —
  the mock files are committed, static Go source, not regenerated.

**Git history / staleness:**
- `tests/kev2/README.md` itself has not been touched since it was added in
  [#38889](https://github.com/rancher/rancher/pull/38889) (2022-09-19) — it is stale
  (wrong Go version, describes `mockcompose` as if actively used).
- The actual test/mock code in `pkg/controllers/management/{aks,eks,gke}` has commits as
  recently as 2026-04-02, but recent changes are mechanical upkeep, not new test coverage:
  - 2026-04-02 (#54206) — updated mocks to match a `Cluster` status-subresource refactor.
  - 2026-01-02 (#53117) — linter fixes.
  - 2024-09-17 (#47128) — migrated off `github.com/golang/mock/gomock` to `go.uber.org/mock`.
  - 2024-10-01, 2024-05-15, 2024-04-10 — dependency bumps.
- The last commits that added or changed actual test *scenarios/fixtures* were earlier:
  GKE autopilotConfig support (2024-01-10, #212729d0), EKS `ebsCsiDriver` field
  (2023-07-10), AKS `outboundType` field (2023-05-05), and operator version bumps
  (2022–2024). So while the tests are exercised on every PR, they have not gained new
  scenario coverage in roughly 2 years — changes since then have been keeping them
  compiling/passing through refactors and dependency upgrades, not expanding what's tested.

### `tests/pkg/serviceaccounttoken/mitigation_test.go` — SA secret creation race/optimistic-locking test

**What it is.** A single-test Go file, `TestOptimisticLocking`, living in its own
`tests/pkg/serviceaccounttoken` package (separate from the unit tests in
`pkg/serviceaccounttoken/*_test.go`) because it needs a real API server + etcd via
[`envtest`](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) rather than
fakes, and the repo's convention (see `tests/controllers/README.md`) is to keep envtest-based
integration tests out of the `pkg` tree.

**What's tested.** `serviceaccounttoken.EnsureSecretForServiceAccount` (`pkg/serviceaccounttoken/secret.go`)
is the function Rancher uses to make sure a `ServiceAccount` has exactly one token `Secret`.
The test regression-guards against a real production bug where concurrent callers (multiple
controller workers racing on the same SA) could each miss the client-side cache and each
create their own secret. It:
1. Starts a real (local) API server via `envtest` and creates a `ServiceAccount`.
2. Spawns a fake background "controller" goroutine (`fakePopulateSecret`) that watches for
   secrets labeled for that SA and populates them with a fake token — standing in for the
   real Kubernetes service-account-token controller.
3. Fires 10 concurrent goroutines that all call `EnsureSecretForServiceAccount` for the same
   SA at once.
4. Asserts only **one** Secret ever exists, the SA is annotated pointing at it, and all 10
   goroutines that got a non-empty UID back agree on the same UID — i.e. optimistic-locking/
   conflict-retry logic in `secret.go` correctly collapses concurrent creators onto a single
   secret instead of creating duplicates.

**Files used.** Just the one file — no fixtures. It exercises the real
`pkg/serviceaccounttoken/secret.go` code path directly (not mocked) against a live
API server.

**Test infrastructure needed.** [`envtest`](https://book.kubebuilder.io/reference/envtest.html)
— a real `kube-apiserver` + `etcd` binary pair, no kubelet/controllers. Requires the
`setup-envtest` tool and a `KUBEBUILDER_ASSETS` env var pointing at downloaded API
server/etcd binaries for a specific Kubernetes version (documented in
`tests/controllers/README.md`, which covers the sibling `tests/controllers` envtest suite).
This is heavier than a pure unit test but far lighter than the full `tests/v2/integration`
suite (no full Rancher server, no k3s cluster).

**Where it's referenced / run — likely a gap.** This is the notable finding: I could not
find any CI workflow or script that actually executes this test.
- It is **not** covered by `.github/workflows/unit-test.yml` (`go test ... ./pkg/...` only
  matches the `pkg` tree, not `tests/pkg`).
- It is **not** covered by `tests/controllers/run_controller_tests.sh`, which is hardcoded to
  `go test -v $(dirname "$0")/...` — i.e. only `tests/controllers/...`, not `tests/pkg/...` —
  even though `.github/workflows/controller-test.yml` (which runs that script) is the only
  workflow that sets up the `envtest` binaries this test needs.
- It is **not** covered by `scripts/test` (unit tests scoped to `./pkg/...`, integration
  tests scoped to `./tests/v2/integration/...`) or `scripts/provisioning-tests`
  (`./tests/v2prov/tests/...`).
- No other workflow greps for `tests/pkg` or uses a repo-wide `./...` test pattern.
- Practically: this test can only be run locally/manually today (`export
  KUBEBUILDER_ASSETS=$(setup-envtest use -p path)` then `go test
  ./tests/pkg/serviceaccounttoken/...`) — it does not appear to gate PRs or pushes.
  **Worth confirming with the team** in case there's a runner I missed (e.g. a
  Drone/Zuul pipeline outside `.github/workflows`), since as documented this looks like
  regression coverage for a real prior production bug that isn't actually enforced in CI.

**Git history / staleness.** Created 2024-11-08 in
[#47273 "Cache mitigation"](https://github.com/rancher/rancher/pull/47273) — the PR that
fixed the underlying secret-duplication bug this test guards against. Subsequently touched
only to track API changes in the code under test, not to add new scenarios:
- 2024-12-17 (#48422) — updated the call to drop a now-unnecessary `sa.DeepCopy()`, alongside
  a real fix to `secret.go`'s update-conflict handling.
- 2025-01-16 (#48729) — mechanical `go vet` fixes.
- 2025-10-16 (#52222) — mechanical signature update (`clientSet` → `clientSet.CoreV1()`,
  `clientSet.CoreV1()`) as part of an impersonation-cache refactor.
No commits since creation have added additional scenarios/assertions — it remains the single
`TestOptimisticLocking` test from its original PR, ~1.7 years old with only interface-churn
maintenance, and (per the above) with no confirmed CI execution path.

### `tests/testdata` — shared Helm chart / UI-extension fixtures for catalog tests

**What it is.** A small, hand-built fake Helm repository plus a fake UI-extension plugin
package, shared as fixture data across both integration tests (`tests/v2/integration/catalogv2`)
and plain Go unit tests (`pkg/catalogv2/oci`). It is *not* Go `testdata/` magic (no package
auto-excludes it since consumers live in different directories) — it's just a conventionally
named fixtures folder referenced by relative path (`../../../testdata/...`).

**What's in it:**
- `index.yaml` — a Helm repo index listing two charts, `testchart` (v1.0.0) and
  `testingchart` (v0.1.0), pointing at the `.tgz` files below. Lets a test stand up a fake
  Helm HTTP repo that looks real to Rancher's `ClusterRepo` code.
- `testchart-1.0.0.tgz`, `testingchart-0.1.0.tgz`, `testingchart-1.0.0.tgz` — actual packaged
  Helm charts (two versions of `testingchart`, used to test upgrade/version-switch behavior).
- `uiext/0.4.1.tgz`, `uiext/files.txt`, `uiext/plugin/{package.json,main.js}` — a fake
  Rancher Dashboard UI extension package: `files.txt` lists the plugin's files
  (`plugin/package.json`, `plugin/main.js`), mirroring the manifest format the real UI
  extension endpoint serves, so a test HTTP server can imitate a plugin registry both in
  "compressed tarball" and "raw file server" modes.

**Where it's used:**
- `tests/v2/integration/catalogv2/cluster_repo_test.go` — `StartHTTPRepository()` serves the
  whole directory (`index.yaml` + chart tgz's) over `httptest.Server` to drive
  `ClusterRepoTestSuite.TestHTTPRepo`; `AddHelmChart()` pushes the same tgz's into a fake OCI
  registry (via `oras`) for the `TestOCIRepo*` and `TestOCIRepoChartInstallation` cases —
  covering HTTP-repo, OCI-repo, multi-repo, and chart-upgrade (`0.1.0` → `1.0.0`) scenarios.
- `tests/v2/integration/catalogv2/ui_plugin_test.go` — `StartUIPluginTgzServer()` and
  `StartUIPluginServer()`/`StartUIPluginServerWithBackoff()` serve `uiext/0.4.1.tgz` or the
  raw `uiext/` directory to drive `TestUIPluginSuite`, exercising Rancher's `UIPlugin`
  CRD reconciliation (compressed vs. endpoint-based install, and retry/backoff on server
  errors).
- `pkg/catalogv2/oci/oci_test.go` and `pkg/catalogv2/oci/client_test.go` — plain unit tests
  read `testingchart-0.1.0.tgz`/`testingchart-1.0.0.tgz` directly off disk to build OCI
  manifests/layers for testing index generation and the OCI registry client, without needing
  a running integration environment.

**Test infrastructure needed.** None to host the fixtures themselves (static files, read
straight off disk or served via Go's own `httptest`/`http.FileServer`). But the *consumers*
have different infra needs:
- The `pkg/catalogv2/oci` unit tests need nothing beyond `go test` — covered by
  `unit-test.yml`'s `go test ./pkg/...`.
- The `tests/v2/integration/catalogv2` suite needs a full running Rancher + downstream
  cluster, i.e. the full `scripts/test` flow (build & run Rancher, `integrationsetup`, then
  `go test ./tests/v2/integration/...`), which `integration-tests.yml` drives in CI on every
  PR/push.

**Git history / staleness.** Actively maintained, not stale:
- Helm chart fixtures (`index.yaml`, `testchart`, `testingchart-0.1.0.tgz`) originated
  2024-01-26 (#unknown "add unit tests oci client") and 2024-04-09 ("Add new units for OCI
  implementation").
- 2024-09-25 — integration tests modified to check for a `User-Agent` header (no fixture
  change).
- 2024-12-27 / 2025-01-28 — the `uiext/` fixture set was added for new UI-plugin integration
  tests.
- 2026-04-08 (#54092, "Add option to filter oci tags") — added `testingchart-1.0.0.tgz` (a
  second version of the same chart) specifically to test version-filtering behavior on OCI
  tags. This is the most recent commit anywhere in the audit so far that both touched a
  fixture file *and* added new test coverage, not just mechanical upkeep.

### `tests/v2/integration` — end-to-end API/control-plane integration suite

**What it is (the grander scheme).** This is Rancher's black-box **integration tier**: ~13 Go
test suites (~31 `*_test.go` files, ~14.8k lines) that exercise a *real, running Rancher
server* and a *real downstream Kubernetes cluster* through Rancher's own APIs, with no mocking
of the system under test. Where the unit tier (§ `tests/kev2`) and the envtest controller tier
(`tests/controllers`) isolate individual packages, this tier answers the question *"does
Rancher behave correctly as an assembled system, from a client's point of view?"* It sits one
rung below the full provisioning tier (`tests/v2prov`) and, notably, **reuses that tier's
machinery**: its downstream cluster is created via `tests/v2prov/cluster`'s `cluster.New()`,
which drives Rancher's real v2 provisioning to spin up a nested
[`systemd-node`](https://github.com/rancher/systemd-node) k3s cluster inside the local k3s
cluster inside the runtime container (the README's architecture diagram spells out this
three-level nesting).

**What it's attempting to test.** Broadly, Rancher's management/control-plane surface as
experienced over the API: catalog & Helm behavior (`catalogv2` — ClusterRepo CRUD, OCI repos,
chart install, UI plugins, system/managed charts), the K8s API proxy (`clusters`), projects &
resource quotas (`projects`), RBAC — role/cluster-role template bindings, impersonation,
feature gating (`rbac`), the Steve resource API (`steveapi`), and user/auth/token/service-account
management (`users`, `authconfigs`, `tokens`, `serviceaccount`). Suites split into those that
need only Rancher's built-in `local` cluster and those that require an imported downstream
cluster (`rancher.clusterName`) — the README's *Test Suites* table marks which is which.
(`defaults/` and `actions/` hold shared helpers, not test functions.)

**Which API surface the tests actually use — three, not one.** Every suite builds its client
through shepherd's `rancher.Client` (`github.com/rancher/shepherd/clients/rancher`), which exposes
three distinct API surfaces that tests mix freely:
- `client.Management` — the legacy **Norman `/v3`** API (typed resources like `.GlobalRole`,
  `.Project`, `.ClusterRoleTemplateBinding`). Note the Norman types are shepherd's *own* vendored
  copy (`shepherd/clients/rancher/generated/management/v3`), not `rancher/rancher`'s client package.
- `client.Steve` — the newer **Steve v1** generic Kubernetes-proxy API (`SteveType(...).Create/
  ByID/Delete`, `ProxyDownstream(...)`).
- `client.WranglerContext` — a **wrangler / raw-REST** context; tests use its `.RESTConfig` with
  `rest.HTTPClientFor(...)` to hit endpoints directly, and `serviceaccount` goes further, building
  a real `client-go` clientset (`clientcmd` → `kubernetes.NewForConfig`) against the local cluster.

The distribution is lopsided — **Norman is the default, Steve is confined to three areas:**

| Package | Norman (`.Management`) | Steve (`.Steve`) | Raw k8s / wrangler |
|---|---|---|---|
| `rbac` | 169 | 0 | 0 |
| `workloads` | 21 | 0 | `rest.HTTPClientFor` |
| `settings` | 14 | 0 | `rest` |
| `tokens` | 12 | 0 | `rest` |
| `clusters` | 10 | 0 | `rest` |
| `projects` | 10 | 0 | 0 |
| `authconfigs` | 7 | 0 | 0 |
| `users` | 4 | 0 | 0 |
| `serviceaccount` | 2 | 0 | real `client-go` clientset |
| `catalogv2` | 9 | 32 | dynamic/client-go |
| `steveapi` | 8 | 68 | 3 |
| `actions/` (helpers) | 0 | 7 | 1 |

Almost every package uses Norman for at least its setup/scaffolding (creating users, projects,
clusters), and the entire management-plane surface — RBAC, projects, settings, tokens, auth
configs — is 100% Norman. Steve is used substantively only in `steveapi` (its whole purpose),
`catalogv2` (ClusterRepo CRUD + `ProxyDownstream`), and a few `actions/` helpers — and even those
fall back to Norman for setup. There is **no single API convention**: management-plane tests are
Norman, catalog/Steve tests are Steve, and a handful reach for raw `client-go`/wrangler REST. This
is the same Norman-vs-Steve enforcement-path split documented in §2's builtin-GlobalRole example,
and it sets up the migration question raised in the companion
[Testing Suggestions](./testing-suggestions.md).

**Where it's run in CI.** Via the reusable workflow
[`.github/workflows/integration-tests.yml`](../../.github/workflows/integration-tests.yml)
(`workflow_call` only), invoked as the `integration-tests` job in `pull-request.yml` after
`build-server`/`build-agent`. Key specifics and asymmetries:
- **PR-only.** It gates pull requests but is **not** wired into `push.yml` — unlike the unit
  tests, which run on both PR and push. So a merge to a branch does not re-run this tier.
- **Coarse path gating.** `pull-request.yml`'s only `paths-ignore` entries are
  `tests/v2/codecoverage/**` and `tests/validation/**`; nearly every other change — including
  docs — triggers the full ~35-minute integration run. This is exactly the "everything runs on
  every PR regardless of what changed" cost pattern that
  [#55231](https://github.com/rancher/rancher/issues/55231) (see section 3) raises for the
  provisioning tier, and it applies here too.
- **Runner / cost.** EIO self-hosted `16cpu-linux-x64`, `spot=false`, 60-min job timeout;
  `go test -v -failfast -timeout 30m -p 1 ./tests/v2/integration/...`. `-p 1` (serialized
  packages, to avoid resource conflicts) means the suite cannot parallelize internally, so its
  wall-clock is inherently long. This matches the flakiness note's classification of this tier
  (§4): **Medium–High flake risk**, ~35 min — because it inherits the timing/registry-pull and
  downstream-provisioning surface it shares with `tests/v2prov`.
- **Two entry points, same command.** `scripts/gha/tests` (GHA) and `scripts/test` (`make ci`,
  container path) both build `integrationsetup`, run it to stand up Rancher + a k3d downstream
  and write `config.yaml`, then run the same `go test` invocation.

**Documentation — present and unusually good.** Three READMEs exist
([`README.md`](../../tests/v2/integration/README.md), `setup/README.md`, `steveapi/README.md`).
The top-level README is the strongest test documentation encountered in this audit: it covers
two run methods (full containerized `make ci`, and local iteration against an external Rancher),
a complete `config.yaml` field reference, the env-var contract (`CATTLE_TEST_CONFIG` et al.),
a per-suite table with downstream-required flags, `go test` flag guidance, and the nested-cluster
architecture explanation. It was **refreshed 2026-06-22** (#55675, "Integration test
improvements"), so it is current, not archaeological. Minor drift worth noting: `setup/README.md`
states Go 1.24 while the workflow pins Go 1.26, and example image tags are hard-coded to
`v2.14-head`.

**Git history / staleness — actively maintained and *growing*.** This is the key contrast with
the frozen suites earlier in this audit (`tests/kev2`, `tests/pkg/serviceaccounttoken`): this
tier is not stale at all.
- Steady commit cadence every month for 2+ years; multiple subdirectories touched within the
  last 1–3 months, with `rbac`, `projects`, and `steveapi` as recent as **2026-07-23** (#56132,
  aggregated role-template feature default).
- The dominant 2026-H1 theme is a **migration of a legacy Python integration suite into Go** —
  #54513 (replace RBAC python), #54716 (migrate python), #54911 (migrate RBAC/project python),
  #54913 (replace k8s-proxy python), and #55175 ("Remove python integration tests"), plus
  #54586 (drop the `rancher/rke` dependency). So this suite is *absorbing* coverage that used to
  live elsewhere rather than stagnating — the opposite trajectory from the KEv2 tests.

**Cross-reference discrepancy to flag.** The flakiness note (§4) attributes its five-tier
taxonomy to `rancher/rancher/TESTING.md`, but **no `TESTING.md` exists anywhere in this repo**.
The tier model is real and useful, but its cited source is missing — either the file was removed,
never committed upstream, or the citation is mistaken. Worth confirming before treating
`TESTING.md` as an authoritative reference in the strategy discussion (companion
[Testing Suggestions](./testing-suggestions.md)).

### `tests/controllers` — envtest controller-integration suite

**What it is (the grander scheme).** This is the **missing middle** of Rancher's test pyramid:
a small suite (5 test packages, ~1,360 lines) that registers individual Rancher controllers
against a real Kubernetes control plane provided by
[`envtest`](https://book.kubebuilder.io/reference/envtest.html) — a live `etcd` + `kube-apiserver`
pair with *no* kubelet, scheduler, or other controllers. That places it deliberately between two
tiers already audited: heavier than the unit tests (which reconcile against *fake* clients), far
lighter than `tests/v2/integration` (which needs a whole running Rancher server + downstream
cluster). The value it uniquely provides is testing a controller's reconcile loop against a
*genuine* API server — real CRD registration, real watch/informer behavior, real optimistic
concurrency — without paying for a full Rancher deployment.

**What it's attempting to test.** Per-controller reconciliation behavior for a focused set of
management controllers: the feature controller (`feature` — the canonical example the README
walks through), auth-config handling and token cleanup (`authconfig`), global roles and their
inherited namespace rules (`globalroles`), the OIDC provider (`oidc`), and generalized deferred
registration (`deferRegistration`). A shared `common` package provides the harness primitives —
`RegisterCRDs`, `StartNormanControllers`, `StartWranglerControllers`, `StartWranglerCaches` —
and the suite explicitly supports both **Wrangler** (current) and **Norman** (legacy) controller
styles, reflecting Rancher's in-progress controller-framework migration.

**Where it's run in CI.** Via its own dedicated workflow
[`.github/workflows/controller-test.yml`](../../.github/workflows/controller-test.yml). Its
wiring differs meaningfully from the other tiers:
- **Standalone triggers, not a reusable `workflow_call`.** It fires directly on `pull_request`
  (every PR, no path filter), on `push` to `release/*` branches, and on `workflow_dispatch`.
  Contrast with `tests/v2/integration`, which is PR-only and invoked as a reusable sub-workflow;
  this tier is the rare one that *also* runs on release-branch pushes.
- **Cheap runner.** Plain GitHub-hosted `ubuntu-latest` — no EIO self-hosted machine — which
  matches the flakiness note's (§4) classification of this tier: ~10 min, **Low–Medium** flake
  risk, "local etcd+apiserver only; no network dependency beyond the runner."
- **Execution.** Runs `run_controller_tests.sh`, which installs `setup-envtest` if absent,
  resolves `KUBEBUILDER_ASSETS` for the pinned `ENVTEST_K8S_VERSION` (`1.30`), then runs
  `go test -v $(dirname "$0")/...`. Also runnable locally via `make controller-test`.

**It is the linchpin of the `tests/pkg/serviceaccounttoken` gap.** This is the most important
cross-finding: `controller-test.yml` is the **only** workflow in the repo that provisions the
`envtest` binaries — yet `run_controller_tests.sh` is hardcoded to
`go test $(dirname "$0")/...`, i.e. **only `tests/controllers/...`**. So the sibling envtest
test at `tests/pkg/serviceaccounttoken` (audited above), which needs exactly these binaries,
is *not* swept up despite the infrastructure being present and running. Widening this script's
test pattern (or adding `tests/pkg/...`) is the smallest plausible fix for that earlier-noted
CI gap.

**Documentation — present, developer-focused, partly incomplete.** A single
[`README.md`](../../tests/controllers/README.md) explains envtest, the `setup-envtest`
dependency, both run methods (Makefile and manual `KUBEBUILDER_ASSETS`), and a "Developing
Tests" walkthrough for registering a Wrangler controller. Its notable gap: the **"Norman
Controller" section is a literal `TODO`** — even though the `common` package ships
`StartNormanControllers` and the harness supports that path. The README has not been touched
since **2024-08-06** (#46016, the PR that first added this suite to CI), so it is ~2 years old
and never grew past its initial draft, while the test code kept evolving underneath it.

**Git history / staleness — slow but genuinely alive.** Created 2024-04 (#44766, feature
controller). Cadence is low and sporadic — a handful of commits per year — but it *is* still
gaining real coverage rather than just churning:
- `oidc` (2026-05, #55178), `globalroles` inherited-namespace-rules (2026-04, #53718), and
  `deferRegistration` + `common` (2025-09, #51767) are all substantive additions, not upkeep.
- The oldest artifacts are the README (2024-08, with the Norman `TODO`) and the `authconfig`
  package (last real change 2024-11 — and that change was #47… "Fix flaky `TestTokensCleanup`",
  which directly corroborates §4: even the Low–Medium envtest tier has produced a real flake,
  from controller-reconciliation timing).

**Two staleness discrepancies to flag.**
- **Go-version drift.** `controller-test.yml` pins `SETUP_GO_VERSION: '1.22.*'`, while the rest
  of the repo builds on Go 1.26 (e.g. `integration-tests.yml`'s `GOLANG_VERSION: '1.26'`). This
  tier is compiled/tested four minor Go releases behind everything else.
- **Old pinned actions.** It still uses `actions/checkout@v3.6.0` and `actions/setup-go@v4.4.0`,
  whereas `integration-tests.yml` is on `checkout@v4.3.1` / `setup-go@v5.6.0`. The workflow's
  last real edit was a permissions fix (2026-05, #54994), not a modernization pass — so it has
  drifted from the repo's current CI conventions.

### `tests/v2prov` — full-stack provisioning (v2prov + CAPR) e2e suite

**What it is (the grander scheme).** The apex of Rancher's test pyramid: end-to-end validation
of **v2 provisioning + CAPR** (Cluster API Provider RKE2/k3s) by actually *building real
downstream Kubernetes clusters* and running day-2 operations against them. It is by far the
largest and heaviest suite in this audit — ~6.0k lines of test code plus ~4.6k lines of
supporting framework (~10.6k Go lines total). Crucially, a large share of that framework is
*infrastructure the rest of the test tree reuses*: `systemdnode/` (runs a real node inside a
container via [`systemd-node`](https://github.com/rancher/systemd-node)), `cluster/` (the
`cluster.New()` helper that `tests/v2/integration` borrows to build its downstream), plus
`operations/`, `objectstore/` (S3/snapshot targets), `registry/`, and `nodeconfig/`. In other
words, v2prov is not just a test suite — it's the provisioning **test harness** the integration
tier depends on.

**What it's attempting to test.** Per the README, three functional categories, each split into
**MP** (machine-provisioned, via nodepools) and **Custom** (framework hand-creates
`systemd-node` pods) variants:
- **General** — lightweight invariants (e.g. system-agent version is as expected).
- **Provisioning** — create/delete of v2prov clusters (1–3 nodes, taints, annotations, drain).
- **Operation** — day-2 ops: etcd snapshot create/restore, **disaster recovery** (snapshot →
  delete etcd node → new node → restore), certificate rotation, encryption-key rotation, custom
  data directories, and imported-cluster operations.

Tests follow a strict `Test_<Category>_<TestName>` naming convention *specifically so the CI
matrix can shard them by regex* — the naming is load-bearing for how the suite is parallelized
(README §"Test Naming Format"). The test tree: `custom/`, `machineprovisioning/`, `imported/`
(the largest at ~1.9k lines), `fleet/`, `autoscaler/`, `general/`, `prebootstrap/`.

**Where it's run in CI — a sharded matrix, the most expensive tier by far.** Via the reusable
workflow [`.github/workflows/provisioning-tests.yml`](../../.github/workflows/provisioning-tests.yml),
invoked as the `provisioning-tests` job in `pull-request.yml` (after `build-server`/`build-agent`).
Like the integration tier it is **PR-only** (not wired into `push.yml`). Key specifics:
- **Sharding by regex, not by package.** `fail-fast: false`; each matrix leg sets
  `V2PROV_TEST_DIST` (`k3s`/`rke2`) and a `V2PROV_TEST_RUN_REGEX`. Every leg runs on its own EIO
  `16cpu-linux-x64`, `spot=false` runner via `./scripts/container-run provisioning-tests`
  (`go test ... -timeout 60m ./tests/v2prov/tests/...`).
- **The live matrix has _nine_ legs, not the seven #55231 describes** (see §3): k3s and rke2
  `General|Provisioning|Fleet`; k3s `Operation_*` (all); k3s and rke2 `Imported_Operation_SetD`
  (a snapshot/disable subset); rke2 `Operation_SetA` and `Operation_SetB` (split apart); and k3s
  and rke2 `PreBootstrap` (gated behind `CATTLE_FEATURES: provisioningprebootstrap=true`). The
  two `Imported_Operation_SetD` legs are absent from the issue's write-up — so the optimization
  proposal is already slightly behind the workflow it targets.
- **Results publishing is a separate, decoupled workflow.**
  `publish-provisioning-test-results.yaml` triggers on `workflow_run` completion of "Build Pull
  Request" and writes checks/PR comments; `convert-to-xml` turns the plaintext logs into JUnit
  XML. Failure logs are captured via the `rancherlabs/cowboy` logdump action — reflecting how
  hard these runs are to debug after the fact.

**Cost — this is the tier #55231 (§3) exists to fix.** The issue measures the suite at
**~50–55 min wall-clock and ~175 CPU-min (~2,800 vCPU-min) per PR**, running in full on every PR
regardless of what changed, and proposes a skip/trimmed/full tiering (trimmed ≈ ~16 min /
~65 CPU-min, ~63% reduction) plus a nightly workflow for the heavy legs. The issue's own
per-suite timings put the slowest legs at ~31–35 min (rke2 Operation_SetA/SetB) and the two
`PreBootstrap` legs at ~7–8 min (kept in PR runs because they're already cheap). Disaster
recovery is explicitly kept in the trimmed set: `Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewNode`
(~509s k3s / ~550s rke2) guards etcd-recovery regressions.

**Documentation — thorough on _how_, stale on _what_.** Two docs:
[`README.md`](../../tests/v2prov/README.md) (categories, MP-vs-Custom, naming convention,
local-run env vars `V2PROV_TEST_RUN_REGEX`/`V2PROV_TEST_DIST`) and a short
[`DEBUG.md`](../../tests/v2prov/DEBUG.md) (how to decode the gzip+base64 debug blobs — itself a
tell about how opaque failures are here). The concepts the README documents are still accurate,
but it was **last meaningfully touched 2023-07-05** (#41771) — it predates the entire
`imported/`, `autoscaler/`, and `prebootstrap/` test areas and the nine-leg matrix, so it
describes the framework's shape but not its current breadth.

**Git history / staleness — the most consistently active suite in the audit.** Created 2023-05
(#41459, CAPR etcd-snapshot/rotation tests) and steadily developed ever since — ~15/12/19/16
commits per year across 2023–2026, with real feature work landing continuously:
`imported/` as recently as **2026-07-13** (#55… ops-annotation rename), `fleet/` day-2 etcd
snapshot (2026-06, #55088), `autoscaler/` refactor (2026-05), `machineprovisioning/` Cluster-API
v1beta2 support (2026-02, #53503), and the workflow itself edited **2026-07-10** (#55869, disable
day2ops logic). This suite tracks CAPR/provisioning development in lockstep — the opposite of the
frozen KEv2 tests.

**Flakiness loop closed.** §4's headline live-cluster-timing failure — "context deadline
exceeded" in `day2ops_disable_reenable_multinode_test.go` — is a real file in
`tests/v2prov/tests/imported/`, exercised by the `ImportedDisableMultiNode` term in the k3s/rke2
`Imported_Operation_SetD` matrix legs. This is the concrete instance behind §4's **High** flake
rating for this tier: nested clusters + single-deadline polling on shared 16-CPU runners is
exactly the combination that produces those timeouts, and it corroborates #55231's push to trim
redundant multi-node variants from PR runs.

## 2. Division of Responsibilities

This section maps *who tests what* across Rancher's test estate — not just within this repo, but
across the sibling [`rancher/tests`](https://github.com/rancher/tests) QA/validation repository —
in order to surface **overlapping coverage and redundancy**. The goal here is descriptive: to
show, with a concrete worked example, that the same behavior is in places validated more than
once, in more than one repo, using the same framework. **It deliberately stops short of assigning
any suite or overlap to a specific team** — doing that responsibly requires more research into
each suite's history, ownership, and intent (see the companion
[Testing Suggestions](./testing-suggestions.md)). What follows is the evidence that such a
division is worth drawing.

### Worked example: RBAC coverage overlaps between `rancher/rancher` and `rancher/tests`

Rancher RBAC is validated in two separate suites that turn out to share far more than their name:

- **`rancher/rancher` → `tests/v2/integration/rbac`** — the integration-tier suite audited in §1
  (`TestRTBTestSuite`, ~94k across `rtbs_test.go`, `global_roles_test.go`, `projects_test.go`,
  `default_roles_test.go`, `impersonation_test.go`, `features_test.go`, `etcdbackups_test.go`).
- **`rancher/tests` → `validation/rbac`** — the QA/validation suite (`crtb/`, `grb/`,
  `globalroles/`, `globalrolesv2/`, `clusterandprojectroles/`, `aggregatedclusterroles/`, plus
  resource-scoped dirs `secrets/`, `configmaps/`, `ingress/`, `workloads/`, `certificates/`,
  `psa/`, `snapshotrbac/`).

**They are built the same way.** Both import `github.com/rancher/shepherd` (the same client,
session, and `extensions/users` helpers), and both exercise the live management API rather than
mocks. The local suite hardcodes `downstreamClusterID = "local"` (management plane only, no
downstream); the external suite is config-driven (`client.RancherConfig.ClusterName` →
`GetClusterIDByName`), so a large subset of it targets the *same* management plane. This shared
foundation is not a coincidence — it is a direct result of the 2026-H1 Python→Go migration noted
in §1 (#54513 "Replace rbac python tests", #54911), which pulled shepherd-based validation *into*
`rancher/rancher` on top of what `rancher/tests` already covered.

**Where they genuinely overlap (management-plane webhook/controller contracts).** These are not
merely "similar topics" — they assert the same admission-webhook and controller-reconciliation
behavior, twice, in two repos:

| Behavior under test | `rancher/rancher` `tests/v2/integration/rbac` | `rancher/tests` `validation/rbac` |
|---|---|---|
| Builtin global role is immutable / undeletable | `TestAdminCannotDeleteBuiltinGlobalRole`, `TestBuiltinGlobalRoleOnlyNewUserDefaultEditable` | `globalroles`: `TestDeleteBuiltinGlobalRoleFails`, `TestUpdateBuiltinGlobalRoleFails`, `TestConvertCustomGlobalRoleToBuiltinFails` |
| Global role CRUD (admin-only) | `TestOnlyAdminCanCRUDGlobalRoles` | `globalroles`: `TestCreateGlobalRole`, `TestGetGlobalRole`, `TestUpdateGlobalRole`, `TestDeleteGlobalRole` |
| GRB subject/target validation via webhook | `TestGRBCannotUpdateSubject`, `TestGRBTargetsUserOrGroup`, `TestGRBGlobalRoleMustExist` | `grb`: `TestGlobalRoleBindingUserPrincipalNameAndGroupPrincipalNameWebhookRejectsRequest` |
| Role-template inheritance / locked templates | `TestPRTBRoleTemplateInheritance`, `TestCRTBRoleTemplateInheritance` | `globalrolesv2`: `TestLockedRoleTemplateInInheritedClusterRole`, `TestAddGlobalRoleWithCustomTemplateAndLockRoleTemplate` |
| Invalid/duplicate CRTB rejected or cleaned up | `TestCRTBMustHaveTarget`, `TestCRTBCannotTargetUsersAndGroup`, `TestDeletingPRTBCleansUpLegacyMembershipLabels` | `globalrolesv2`: `TestDuplicateCRTBsAreDeleted`, `TestCRTBWithLocalClusterReferenceIsDeleted`, `TestRoleTemplateWithBadUserSubject`; `crtb`: `TestValidateError*` |
| Default roles/bindings on cluster & project creation | `TestClusterCreateDefaultRole`, `TestProjectCreateDefaultRole`, `TestUserCreateDefaultRole` | `clusterandprojectroles`: `TestClusterRolesOnClusterCreationAndDeletion`, `TestRolesOnProjectCreationAndDeletion` |
| Project owner binding behavior | `TestProjectCreatorGetsOwnerBindings` | `clusterandprojectroles`: `TestProjectOwnerAddsAndRemovesOtherProjectOwners`, `TestClusterOwnerAddsUserAsProjectOwner` |

**Assertion-level illustration (one matched pair).** To confirm the overlap is real behavior and
not just matching names, compare the assertions of the first row's pair — "builtin global role
cannot be deleted":

*Local — `rancher/rancher`, `TestAdminCannotDeleteBuiltinGlobalRole`, via the Norman management
API:*
```go
err = client.Management.GlobalRole.Delete(gr)          // gr = builtin role "admin"
var apiErr *clientbase.APIError
p.Require().True(errors.As(err, &apiErr))
p.Require().Equal(http.StatusForbidden, apiErr.StatusCode)      // HTTP 403
p.Require().Contains(apiErr.Body, "cannot delete builtin global roles")
```

*External — `rancher/tests`, `TestDeleteBuiltinGlobalRoleFails`, via the admission webhook:*
```go
err := rbacapi.DeleteGlobalRole(gr.client, builtinGlobalRoleName)   // builtin role "user"/StandardUser
require.Error(gr.T(), err)
expectedErrMessage := fmt.Sprintf("%s cannot delete builtin GlobalRoles", webhookErrorMessagePrefix)
require.Contains(gr.T(), err.Error(), expectedErrMessage)
_, err = rbacapi.GetGlobalRoleByName(gr.client, builtinGlobalRoleName)
require.NoError(gr.T(), err)                            // role still exists
```

**What the diff shows.** The *invariant* is identical — deleting a builtin global role must fail,
and the role must survive — so this is genuine redundancy of intent, not a naming coincidence.
But the two tests exercise **different enforcement points**: the local test goes through the
Norman management API and asserts an **HTTP 403** with the prose `"cannot delete builtin global
roles"`; the external test goes through the **admission webhook** (`webhookErrorMessagePrefix`,
Kubernetes kind `GlobalRoles`) and additionally re-fetches to prove survival. This is the crux of
why the redundancy is subtle: the same rule is validated at two distinct chokepoints, so a naive
"delete the duplicate" would silently drop coverage of one enforcement path. (The local test also
bundles extra assertions in the same function — builtin flag ignored on create, no `remove` link,
no-op update allowed — whereas the external suite splits those into a separate
`TestUpdateBuiltinGlobalRoleFails`; so even the "same" behavior is sliced differently between the
suites.)

**Where they diverge (each repo's unique coverage).** The overlap is real but partial — each side
also holds coverage the other lacks, which is why this is a *division-of-responsibility* problem
rather than a simple "delete the duplicate" problem:

- **Only in `rancher/tests`:** downstream *resource-scoped* RBAC (secrets, configmaps, ingress,
  certificates, and workloads — cronjob/daemonset/deployment/job/statefulset), Pod Security
  Admission, snapshot RBAC, aggregated cluster roles, `restrictedadmin` + its replacement role
  (absent entirely from the local suite), status-field / `kubectl explain|describe` UX checks,
  and the dynamic-input permission matrix (`TestRBAC` / `TestRBACDynamicInput`).
- **Only in `rancher/rancher`:** namespace-access revocation on PRTB delete
  (`TestRemovingPRTBRevokesNamespaceAccess`), impersonation-by-cluster-role, project *resource
  quota* API validation (the bulk of `projects_test.go`), feature-flag access
  (`TestCannotCreateFeature` / `TestCanListFeatures`), and etcd-backup manage-role checks.

**Why this matters (and what it is *not* saying).** The same management-plane RBAC contracts are
being maintained in two repositories, with the same framework, run by (presumably) different
groups on different cadences — the local set gates every `rancher/rancher` PR (§1), while the
`rancher/tests` set runs as release/QA validation. That is duplicated maintenance surface and a
double source of truth for the same webhook/controller behavior: a webhook change can now break
tests in two repos, or — worse — be covered in one and silently stale in the other. This entry
does **not** claim either suite is wrong, nor that the overlap should collapse to one owner. It
establishes that the redundancy exists and is measurable. Deciding *which* layer should own the
management-plane contract tests (dev integration vs. QA validation), and whether the shared
shepherd foundation should be leveraged to dedupe, is the open question — and needs the deeper
per-suite ownership/intent research called out in the companion
[Testing Suggestions](./testing-suggestions.md) before any work is assigned.

## 3. Related Work

This section summarizes and links other in-flight or completed efforts that touch the same
problem space as this audit — what tests exist, where they live, and how/when they run in CI.
Whereas sections 1–2 focus on *what is covered and by which suite*, the work tracked here is
largely about *test execution economics* (when suites run, on which PRs, and at what compute
cost).

### [#55231](https://github.com/rancher/rancher/issues/55231) — Optimize Provisioning Tests CI: trim PR suites, move heavy tests to nightly

**Type / ownership.** Task, milestone **v2.16.0**, labels `kind/ci-improvements` /
`team/hostbusters`, assigned to HarrisonWAffel and snasovich. Status: open.

**Problem.** The provisioning test suite runs in full on *every* PR — 7 parallel matrix jobs
on 16-CPU runners — regardless of what changed. That is ~175 CPU-minutes (~2,800 vCPU-minutes)
per run, 50–55 min wall-clock, with runner wait times exceeding 1h40m. As PR throughput rises
(partly from agentic coding tooling), this CI cost has become an anchor on development
velocity, even for docs- or dashboard-only changes.

**Proposed three-tier approach.**
- **Tier 1 — skip entirely:** PRs touching only docs, markdown, licenses, build configs,
  validation tests, or dev scripts skip provisioning tests altogether (100% compute savings).
- **Tier 2 — full suite:** PRs touching provisioning-critical paths (`pkg/capr/`,
  `pkg/provisioningv2/`, related controllers, fleet, etcd/certificates) run the complete
  matrix.
- **Tier 3 — trimmed (default):** All 7 suites still run, but with essential tests only —
  basic provisioning, fleet integration, certificate/encryption-key rotation, and one
  disaster-recovery test per distro.

**Expected impact.** Trimmed PR runs drop to ~65 CPU-minutes (~63% reduction); the longest
suite falls from 35–40 min to ~16 min; skip-eligible PRs save 100%.

**Implementation components.**
1. A new nightly scheduled workflow that runs the full suite.
2. Path-based job outputs that decide the test tier per PR.
3. A redefined trimmed matrix with focused regex patterns for k3s and rke2.
4. Disaster-recovery coverage validation, while removing redundant multi-node variants.

**Relevance to this audit.** Directly complementary. This audit inventories the provisioning
and integration suites (`tests/v2/integration`, `tests/v2prov`, `scripts/provisioning-tests`,
etc.) and notes which workflows execute them; #55231 proposes changing *when* those same
suites run per-PR. It also reinforces a theme surfaced in section 1 — that CI execution paths
for tests are uneven (e.g. the `tests/pkg/serviceaccounttoken` gap) — and is a good reference
point when defining the forward-looking options in the companion
[Testing Suggestions](./testing-suggestions.md).

### Flakiness root-cause note — [Why is CI flaky?](./flakiness-report-2026-07-21.md) (2026-07-21)

An AI-assisted root-cause analysis of CI flakiness across `rancher/rancher` and
`rancher/charts`, based on seven hand-sampled failed runs. The full report lives alongside this
audit at [`flakiness-report-2026-07-21.md`](./flakiness-report-2026-07-21.md); its findings are
also summarized in **section 4** of this document (it is a dated, external analytical artifact
rather than a code change, so it is cross-referenced here alongside #55231). Its headline contribution is a
reframing — "flaky" conflates at least four distinct problems (non-deterministic tests,
infra/network flakiness, CI-platform flakiness, and correct rejections that merely *look* like
flakes in the pass/fail metric) — and a tier→flake-risk mapping that directly informs the
tiering strategy option in the companion [Testing Suggestions](./testing-suggestions.md).

## 4. Test Flakiness

**Source.** This section integrates an AI-assisted root-cause note,
[*"Why is CI flaky? rancher/rancher & rancher/charts"*](./flakiness-report-2026-07-21.md)
(evidence pulled **2026-07-21** via `gh run view --log-failed`, read-only). The full report is
kept alongside this audit at [`flakiness-report-2026-07-21.md`](./flakiness-report-2026-07-21.md);
this section is a condensed, cross-linked summary of it. It is not an exhaustive audit — seven failed runs were
hand-sampled across two batches from recent workflow history in both repos; aggregate fail-rate
numbers come from a 100-run-per-repo snapshot. Treat the specifics as illustrative of *classes*
of flakiness, not a complete census.

### The core reframing: "flaky" is at least four different problems

The single most useful takeaway is that a raw CI "failure rate" lumps together causes with
completely different owners and fixes. Distinguishing them is what breaks the retry-and-hope
cycle:

- **Test flakiness** — the test itself is non-deterministic (map-iteration order, single
  deadlines under load, races between a setup step and the assertion that depends on it). *Owner:
  test authors.*
- **Infra flakiness** — a transient failure in infrastructure the test depends on but doesn't
  control (a CDN connection reset mid-download, concurrent git writers racing on the same ref).
  *Owner: pipeline/infra, usually via retry-with-backoff.*
- **Platform flakiness** — a shared CI-platform service failing (e.g. a Vault token mint
  returning 404). Not a test problem at all; affects every repo using that integration. *Owner:
  the platform team.*
- **Metric artifact** — a *correct* rejection (drift detector catching an out-of-process chart
  change) that counts against "failure rate" identically to a real flake. *Nothing to fix in
  the test; the metric is what's misleading.*

Of the seven sampled runs: 4 test flakiness, 1 infra, 1 platform, 1 metric artifact (a correct
rejection). This is why an open item like "distinguish flaky (fails on re-run of same commit)
from broken (fails consistently)" matters — the headline flake rate overstates real flakiness.

### The seven sampled failures

| Category | Suite / workflow | Root cause | Suggested fix |
|---|---|---|---|
| Test — non-deterministic assertion | `rancher` unit-tests (`TestArbitrarySign_WithCredential`) | String built from Go map iteration (randomized), compared to a fixed expected string | Sort/iterate fields deterministically before comparing |
| Test — live-cluster timing | `rancher` provisioning (v2prov) `day2ops_disable_reenable_multinode` | Single deadline hit before agent reported uninstall complete; recurred twice in one day | Raise wait budget or poll-and-retry instead of single-deadline |
| Test — permission-propagation race | `rancher` integration RBAC (`TestProjectCreatorGetsOwnerBindings`) | Impersonated list ran before the owner role binding propagated to the authorizer | Poll an authorized call until the binding takes effect |
| Infra — upstream network | `charts` Build/Validate | `release-assets.githubusercontent.com` CDN reset connection mid-download | Retry-with-backoff around the archive fetch |
| Infra — concurrent writers | `charts` Manual Auto Bump | Second auto-bump push lost the race; branch moved between fetch and push | Retry with fetch+rebase on push rejection |
| Platform — shared secrets | `rancher` Renovate | Vault token mint returned 404, twice in a day | Not a repo fix; report to Vault/GHA integration owner |
| Metric artifact (not a flake) | `charts` Build/Validate | Drift detector correctly caught `rancher-backup-crd 3.0.0` removed without updating `release.yaml` | None — correct rejection; fix the metric, not the job |

### Flake risk by test tier

This is the finding that ties most directly to the rest of the audit. Flake risk tracks
*setup cost* almost exactly — the more a test depends on live infrastructure it doesn't fully
control, the more ways it can fail for reasons unrelated to the code under test. This maps onto
the same five-tier taxonomy (`TESTING.md`) that section 1 inventories suite-by-suite:

| Tier | Live cluster? | Setup cost | Flake risk | Why |
|---|---|---|---|---|
| Unit (`pkg/**/*_test.go`) | No | Trivial | **Low** | Fully isolated; failures are real bugs or non-deterministic test code |
| Controller integration (`tests/controllers/`) | No (envtest) | Low–Medium | **Low–Medium** | Local etcd+apiserver only; no network beyond the runner |
| Integration v2 (`tests/v2/integration/`) | Yes (Rancher + k3d) | High | **Medium–High** | Real Rancher + downstream cluster; timing and registry-pull deps enter |
| Provisioning v2prov (`tests/v2prov/`) | Yes (full stack) | Very high | **High** | Nested clusters, systemd-node, 8-way k3s/rke2 matrix — the "context deadline exceeded" case lives here |
| KEv2 hosted (`tests/kev2/`) | No (mocked) | Medium, self-contained | **Low** | Mocked cloud APIs by design — traded real coverage for determinism on purpose |

Note how this cross-checks section 1: the KEv2 suite's low flake risk is the flip side of the
staleness/coverage observation there (mocking buys determinism at the cost of real-world
coverage), and the high-flake provisioning tier is exactly the suite #55231 (section 3) targets
for trimming.

### Structural findings worth carrying into strategy

- **Flakiness is currently paid for with automation, not measured or fixed.** Both repos ship a
  dedicated *"Retry failed jobs once"* workflow (52 runs on charts, 11 on rancher in the sample).
  Auto-retry hides flakiness from the metric while leaving the root cause in place.
- **`rancher/charts` has two structural flakiness sources `rancher/rancher` lacks**, because it
  packages/validates ~40 upstream projects in one job: (1) *network fan-out* — every touched
  chart's `.tgz` is fetched from a CDN, multiplying exposure to transient resets; and (2) *drift
  detection reading as failure* — a correct rejection is indistinguishable from a flake in the
  raw pass/fail signal.
- **The expensive tiers already opt out of spot pricing** (`spot=false` on unit, integration,
  and provisioning) to avoid losing a 30–35 min run to a spot interruption — a deliberate,
  already-made cost/reliability trade-off worth preserving in any tiering redesign.
- **Whole-run wall-clock is misleading once a re-run happens** (one umbrella run showed 391 min
  end-to-end but had a ~5-hour gap from a manual re-run of failed legs). Per-job durations are
  the reliable number for reasoning about cost.

### Implications for this audit

1. **Signal quality is a prerequisite for the tiering strategy** (companion
   [Testing Suggestions](./testing-suggestions.md)). Before deciding
   which suites to trim or move to nightly, the flaky-vs-broken distinction must be closeable —
   otherwise a "flake" that is really a broken high-flake test gets quietly demoted to nightly.
2. **Each flakiness class needs a different owner/fix**, so remediation shouldn't be filed as one
   "reduce flakiness" task — the four categories above split cleanly across test authors, infra,
   and the platform team.
3. **Flake risk is a scoring input for tier assignment**, alongside the path-based mapping in
   the companion [Testing Suggestions](./testing-suggestions.md) and the coverage/staleness
   findings in section 1.
