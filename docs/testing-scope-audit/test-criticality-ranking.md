# tests/e2e Test Criticality Ranking

Source: [`tests/e2e/test-summary.md`](../../tests/e2e/test-summary.md) (generated via `go generate ./tests/e2e`).
This ranking was produced from that summary document alone — no test implementations were read directly.

## Methodology

Each test is scored on:
1. **Blast radius of a regression** — e.g. losing the ability to log in or access a cluster
   outranks losing the ability to create one specific secret type.
2. **Redundancy** — when several tests in the same file exercise the same underlying mechanism
   with only a parameter changed (e.g. "install with toleration A" vs. "install with toleration B"),
   the first/most-representative test is scored higher and later variants are marked down, since
   the marginal coverage they add is smaller.
3. **Holism vs. implementation detail** — a test that drives a full user-facing action (create a
   project, install a chart) ranks above a test that only checks an implementation detail (a schema
   field's create/update permission, a response header).

Tiers: **Critical** (core to Rancher functioning at all — login, cluster/project access, privilege
boundaries) > **High** (an important, broadly-used feature or a real security boundary) > **Medium**
(real correctness/security value but narrower scope) > **Low** (implementation detail, redundant
variant, or narrow edge case) > **Irrelevant** (negligible impact if regressed — not used below;
nothing in this suite qualified).

Cross-reference: the companion [testing-scope-audit.md](./testing-scope-audit.md) documents that
`rancher/tests` (the separate QA/validation repo) duplicates a large share of this repo's `rbac/`
coverage at the same management-plane chokepoints — worth keeping in mind before treating a
"Critical" rating here as "uniquely irreplaceable," since some of this is a second copy of a
control that's also gated elsewhere.

---

## authconfigs/

| Test | Tier | Why |
|---|---|---|
| `TestAuthConfigsExistAndCannotBeDeleted` | High | Protects the auth-provider list itself and blocks deleting configs outright — losing this could silently disable login for an org. |
| `TestAuthConfigActions` | Medium | Checks which UI actions each type exposes — wrong metadata degrades UX, doesn't block auth. |
| `TestAuthConfigSecrets` | High | Verifies the controller actually materializes the secret backing a configured provider (e.g. SAML) — if this breaks, that provider silently fails at login time. |

## catalogv2/

### charts_test.go
Feature under test is CP-node-taint toleration handling during install/upgrade/uninstall, plus a
schema-validation bypass. Chart install itself is broadly important, but this specific dimension
(tainted control-plane nodes) is a narrower HA scenario, and 7 of 9 tests are minor variations of
the same install→check-taint pattern.

| Test | Tier | Why |
|---|---|---|
| `TestInstallChartWithAutomaticTolerationOnTaintedCPNode` | Medium | First occurrence of the core install-with-toleration assertion. |
| `TestInstallChartWithCustomTolerationOnTaintedCPNode` | Low-Medium | Same pattern as above, custom instead of automatic — marginal value once the first is covered. |
| `TestUpgradeChartWithCustomTolerationOnTaintedCPNode` | Medium | Adds the upgrade action (not just install) to the toleration check — a distinct holistic operation. |
| `TestUpgradeChartWithAutomaticTolerationOnTaintedCPNode` | Low | Near-duplicate of the above, only automatic vs. custom differs. |
| `TestUpgradeChartInstalledWithoutTolerationsUsingAutomaticTolerations` | Low | Another toleration-combination variant of the same upgrade path. |
| `TestUninstallChartWithAutomaticTolerationOnTaintedCPNode` | Medium | Covers the uninstall action — a distinct holistic operation from install/upgrade. |
| `TestUninstallChartWithCustomTolerationOnTaintedCPNode` | Low | Redundant with the above, toleration-mode variant only. |
| `TestInstallChartWithSkipSchemaValidation` | Medium | Distinct feature — bypassing schema validation matters for oversized/nonstandard charts. |
| `TestUpgradeChartWithSkipSchemaValidation` | Low-Medium | Same feature as above applied to upgrade instead of install. |

### cluster_repo_test.go
ClusterRepo is the backbone of the whole catalog system — nothing installs without a working repo — so this file skews High, with repeated variants dropping.

| Test | Tier | Why |
|---|---|---|
| `TestHTTPRepo` | High | Baseline CRUD for the most common repo type — breaking this breaks Apps & Marketplace for HTTP repos. |
| `TestGitRepo` | High | Same CRUD guarantee for Git-backed repos (e.g. `rancher/charts` itself) — a genuinely different code path from HTTP. |
| `TestGitRepoRetries` | Medium | Narrower — just confirms retry/backoff behavior, a resilience detail rather than a new capability. |
| `TestOCIRepo` | High | CRUD for OCI repos — a third, increasingly-used distribution mechanism. |
| `TestOCIRepo2` | Low-Medium | Same OCI CRUD pattern with a different path/tag layout — largely redundant with `TestOCIRepo`. |
| `TestOCIRepo3` | Medium | Confirms 4xx errors surface correctly and aren't endlessly retried — prevents masking real permission/not-found errors as flakes. |
| `TestOCIRepo4` | Medium | Confirms 429 rate-limit backoff actually recovers — meaningful resilience behavior against public registries. |
| `TestOCIRepo5` | Low | Same 429 scenario as above with a different header variant — marginal incremental value. |
| `TestOCIRepoMultipleChartRepos` | Low | A scale check (300 charts) on the CRUD path already covered by `TestOCIRepo` — more a perf sanity check than new behavior. |
| `TestOCIRepoWithOptions` | Medium | Tag-filter option is a distinct, user-facing configuration knob. |
| `TestOCIRepoChartInstallation` | High | The actual end-to-end "install a chart from an OCI repo" holistic action — arguably the most valuable test in the file. |
| `TestOCIEnableRepo` | Medium | Enable/disable toggle is a real feature, but narrower than full CRUD. |

### rancher_managed_charts_test.go
Governs Rancher's self-management of its own critical addons — a regression can leave Rancher's own subsystems undeployed or stuck.

| Test | Tier | Why |
|---|---|---|
| `TestInstallChartLatestVersion` | High | Confirms Rancher installs the latest version of a managed system chart — core to keeping Rancher's own components current. |
| `TestUpgradeChartToLatestVersion` | High | Confirms recovery to the correct latest version after an index regression — protects against a bad index permanently stranding Rancher on the wrong version. |
| `TestUpgradeToWorkingVersion` | Medium-High | Degraded-version handling and recovery — important, but a narrower scenario than the two above. |
| `TestUpgradeToBrokenVersion` | Medium-High | Mirror of the above for a different broken-version case; some redundancy with it in the revert logic. |
| `TestServeIcons` | Low | Only asserts a clone directory gets created — a shallow proxy assertion, not that icons are actually served correctly. |

### system_charts_version_test.go

| Test | Tier | Why |
|---|---|---|
| `TestInstallWebhook` | Critical | `rancher-webhook` backs most admission/validation logic across the whole product — if it can't install at the pinned version, large parts of Rancher (including much of the RBAC enforcement this very suite tests) break. |
| `TestInstallFleet` | High | Fleet is a major, widely-used Rancher subsystem, though not as universally load-bearing as the webhook. |

### ui_plugin_test.go
Affects the Dashboard's extensibility system, not the control plane — real but bounded blast radius.

| Test | Tier | Why |
|---|---|---|
| `TestGetIndexAuthenticated` | Medium | The primary path most users hit — plugin index reflects installed plugins. |
| `TestGetIndexUnauthenticated` | Medium | Auth-gating on the index itself, protecting non-public plugin metadata. |
| `TestCorrectContentType` | Low | One header value on one file — low blast radius if wrong. |
| `TestGetSingleExtensionAuthenticated` | Low-Medium | Basic file-serving happy path, narrower than the index tests. |
| `TestGetSingleExtensionUnauthenticated` | Low-Medium | Same as above for the unauthenticated case. |
| `TestGetSingleUnauthorizedExtension` | Medium | A security-boundary check — an auth-required plugin file must not leak to anonymous users. |
| `TestCompressedEndpoint` | Low | Narrow packaging-format support check. |
| `TestExponentialBackoff` | Low | Retry-timing detail, not a functional capability. |
| `TestUnreachableCompressedEndpoint` | Low | Fallback-behavior edge case, narrow scope. |

## clusters/

| Test | Tier | Why |
|---|---|---|
| `TestImportInitialConditions` | Low | Checks one narrow initial-state invariant (no conditions at creation) — object-shape detail, not functional behavior. |
| `TestClusterNodeCount` | High | Node-count tracking is widely relied on (UI, alerts, scaling decisions) — a regression gives operators wrong information about their cluster. |
| `TestK8sProxyFetchesNamespacesFromLocalCluster` | Critical | The k8s proxy is how Rancher's UI/kubectl reach cluster APIs through Rancher at all — a foundational, extremely widely-used capability. |
| `TestK8sProxyFetchesNamespacesFromDownstreamCluster` | Critical | Same capability as above for downstream clusters — arguably more critical since most real-world usage targets downstream, not local. |
| `TestProxyK8sV1PathReturnsNotFound` | Low | Negative-path/edge-case check on a malformed path. |
| `TestNodeFields` | Low | Norman schema field-permission metadata — implementation detail, not behavior. |
| `TestNodeDriverSchema` | Medium | A real security check — prevents sensitive local filesystem paths leaking through node-driver schemas. |
| `TestAmazonNodeDriverSchema` | Low | Single-field presence check on one cloud provider's schema. |
| `TestCannotCreateAzureNoAccountStorageType` | Medium | Real storage-provisioning guard rail preventing broken PVCs — narrow to one cloud/storage-class combination. |
| `TestCanCreateAzureAnyAccountStorageType` | Low-Medium | Positive-path counterpart to the above, same narrow scope. |
| `TestCanCreatePVCNoStorageNoVol` | Medium | PVC creation without a storage class is a common, broadly-used path. |
| `TestPersistentVolumeUpdate` | Medium | Real data-integrity guard rail (immutable PV source type), though direct PV editing is a less common workflow than PVC creation. |

## projects/

| Test | Tier | Why |
|---|---|---|
| `TestCreateNamespaceProjectMember` | High | Namespace creation inside a project is one of the most common, everyday actions in Rancher. |
| `TestCreateNamespaceProjectOwner` | High | Same holistic capability as above for a different role — some overlap, but validates a different RBAC binding path. |
| `TestCreateNamespaceWithQuotaInProject` | High | Resource quotas are a core multi-tenancy safety feature — verifies the default actually propagates. |
| `TestCreateNamespaceWithOverriddenQuotaInProject` | Medium | Narrower variant (override + exceeding-limit clamp) building on the same foundation as above. |
| `TestRemoveQuotaFromProjectWithNamespacePropagation` | Medium | Quota removal propagation — a secondary flow relative to quota creation. |
| `TestAddQuotaFromProjectWithNamespacePropagation` | Medium | Same propagation mechanism as above in the opposite direction. |

## rbac/

### default_roles_test.go
Underpins automatic RBAC bootstrapping — if this regresses, newly created clusters/projects/users can end up with no (or wrong) access with no manual recourse. A severe, silent failure mode.

| Test | Tier | Why |
|---|---|---|
| `TestClusterCreateDefaultRole` | Critical | Without this, cluster creators could be locked out of clusters they just made. |
| `TestClusterCreateRoleLocked` | High | Narrower variant covering the "locked role" exclusion rule on top of the same mechanism. |
| `TestProjectCreateDefaultRole` | Critical | Same guarantee as cluster creation, for projects — one of the most common actions in Rancher. |
| `TestProjectCreateRoleLocked` | High | Locked-role variant of the above, same relationship. |
| `TestUserCreateDefaultRole` | High | New users getting default global roles affects every onboarding, though triggered by a narrower CRTB-driven flow. |
| `TestDefaultSystemProjectRole` | Medium | Checks a static, rarely-changing invariant — important but unlikely to regress independent of the above. |

### etcdbackups_test.go

| Test | Tier | Why |
|---|---|---|
| `TestBackupsManageRole` | Medium-High | etcd backups are critical for disaster recovery; this protects the RBAC gate over who can manage them — narrow but real. |
| `TestStandardUsersCannotAccessBackups` | Medium-High | The security-boundary counterpart — a leak here exposes backup control to unauthorized users. |

### features_test.go

| Test | Tier | Why |
|---|---|---|
| `TestCannotCreateFeature` | Medium | Feature flags can't be mutated via the API even by admins — a narrow but real guard rail against flag corruption. |
| `TestCanListFeatures` | Low-Medium | Read access for standard users — low risk if this regressed (a UI read failure, not a security break). |

### global_roles_test.go
Global roles are a foundational RBAC building block — incorrect enforcement can mean privilege escalation or lockout.

| Test | Tier | Why |
|---|---|---|
| `TestUserVsUserBaseGlobalRoleVisibility` | High | A regression could leak the entire user list/role-template list to lower-privileged users — real information disclosure. |
| `TestKontainerDriverVisibilityByGlobalRole` | Low-Medium | Narrow visibility check for one resource type tied to specific roles. |
| `TestBuiltinGlobalRoleOnlyNewUserDefaultEditable` | High | Protects builtin roles (e.g. "admin") from silent modification — a real escalation/lockout vector. |
| `TestOnlyAdminCanCRUDGlobalRoles` | Critical | The core authorization boundary for who can manage roles at all — if standard users could CRUD global roles, they could grant themselves admin access. |
| `TestAdminCannotDeleteBuiltinGlobalRole` | High | Prevents deleting a builtin role like "admin," which would lock out every admin — high impact, narrower than the blanket CRUD boundary above. |
| `TestGRBCannotUpdateGlobalRoleID` | Medium-High | Prevents repointing a binding to a more-privileged role post-creation — a real escalation vector, but a field-level immutability check. |
| `TestGRBGlobalRoleMustExist` | Medium | Guard rail against dangling/bogus bindings — real but lower stakes than the mutation-prevention tests. |
| `TestGRBCannotUpdateSubject` | Medium-High | Prevents re-targeting a binding to a different user/group after creation — same risk class as the GlobalRoleID test. |
| `TestGRBTargetsUserOrGroup` | Low-Medium | Input-validation guard rail against malformed subject combos. |

### impersonation_test.go

| Test | Tier | Why |
|---|---|---|
| `TestImpersonationByClusterRole` | High | Impersonation is a powerful, security-sensitive capability; this covers both the grant and the restrict-to-target case in one holistic test. |

### projects_test.go
Mixes genuine RBAC security boundaries with narrower quota-API field/validation checks.

| Test | Tier | Why |
|---|---|---|
| `TestProjectCreatorGetsOwnerBindings` | High | Holistic, real-world flow — a user creates a project and can actually work in it. |
| `TestReadOnlyCannotEditSecret` | High | Core negative-path RBAC boundary — a regression means "read-only" users can tamper with secrets. |
| `TestReadOnlyCannotMoveNamespace` | Medium-High | Narrower privilege-boundary check, same risk class as above but a less common action. |
| `TestSystemProjectCreated` | Low | Static invariant (labels on well-known projects) — unlikely to regress independently. |
| `TestSystemProjectCannotBeDeleted` | Medium | Protects a foundational system object from deletion, though rarely a real-world deletion target. |
| `TestSystemNamespacesDefaultServiceAccount` | Medium | Real security hardening check (no auto-mounted SA tokens in system namespaces) — narrow surface. |
| `TestProjectResourceQuotaFields` | Low | Basic field round-trip check — implementation detail. |
| `TestProjectQuotaAPIValidation` | Medium | Input-validation guard rails across several invalid quota shapes. |
| `TestProjectContainerDefaultResourceLimit` | Low-Medium | Narrow field-level CRUD check for one project setting. |
| `TestNamespaceResourceQuotaCreated` | Medium | Confirms explicit per-namespace quota actually creates the underlying k8s object. |
| `TestNamespaceDefaultQuotaApplied` | Medium | Same mechanism as above via default-inheritance instead of explicit annotation. |
| `TestProjectUsedQuotaUpdated` | Medium | Usage accounting correctness — needed for quota enforcement to mean anything, but a bookkeeping detail. |
| `TestProjectQuotaUpdateAppliedToNamespace` | Medium | Confirms retroactive quota application to pre-existing namespaces — closes a real gap. |
| `TestProjectUsedQuotaExactMatch` | Low-Medium | Edge-case (exact-match boundary) variant of the usage-accounting tests above. |
| `TestProjectQuotaAddRemoveFields` | Low-Medium | Narrow field-add/remove propagation check, incremental on existing quota coverage. |
| `TestProjectQuotaCannotExceedWithExistingNamespaces` | Medium | Real guard rail preventing an impossible quota given existing usage. |
| `TestNamespaceQuotaExceedsProjectLimit` | Low-Medium | Edge-case variant overlapping conceptually with the override test in `resource_quota_test.go`. |

### rtbs_test.go
Role-template binding mechanics — the connective tissue that makes RBAC assignments actually take effect.

| Test | Tier | Why |
|---|---|---|
| `TestPRTBRoleTemplateInheritance` | High | Role-template inheritance (composing custom roles from other roles) is widely used — if it breaks, real custom-role setups silently gain or lose access. |
| `TestCRTBRoleTemplateInheritance` | High | Same mechanism at cluster scope, including chained multi-level inheritance. |
| `TestRemovingPRTBRevokesNamespaceAccess` | High | Confirms revocation actually works — a regression means former members retain access they shouldn't, a real security leak. |
| `TestAPIGroupInRoleTemplate` | Medium-High | Confirms scoped API-group permissions (get/list/watch but not delete) are enforced — a genuine least-privilege boundary. |
| `TestDeletingPRTBRemovesClusterAccess` | High | Same revocation guarantee as above via the underlying ClusterRoleBinding mechanism — meaningful but overlapping intent. |
| `TestDeletingPRTBCleansUpLegacyMembershipLabels` | Medium | Narrower variant focused on a legacy-label migration path — matters for upgrades, affects a smaller population. |
| `TestCRTBCannotTargetUsersAndGroup` | Low-Medium | Input-validation guard rail against a malformed binding. |
| `TestCRTBMustHaveTarget` | Low-Medium | Same validation class as above (missing subject instead of dual subject). |
| `TestCRTBCannotUpdateSubjectsOrCluster` | Medium-High | Prevents a binding from being silently repointed post-creation — same risk class as the GRB immutability tests. |

## serviceaccount/

| Test | Tier | Why |
|---|---|---|
| `TestSingleSecretForServiceAccount` | High | Guards a real, previously-shipped production bug — concurrent secret creation causing duplicates breaks service-account auth for downstream workloads. Note: [testing-scope-audit.md](./testing-scope-audit.md) documents a near-identical `tests/pkg/serviceaccounttoken.TestOptimisticLocking` guarding the same function, with no confirmed CI path — this e2e copy may be the one actually enforced. |

## settings/

General settings CRUD + read-only protection — broad but shallow blast radius (surfaces quickly if broken).

| Test | Tier | Why |
|---|---|---|
| `TestCreateReadOnly` | Medium | Protects a system-managed setting (cacerts) from being overridden via create. |
| `TestUpdateReadOnly` | Medium | Same protection as above for the update path. |
| `TestGetReadOnly` | Low | Simple read-path happy case, low risk. |
| `TestDeleteReadOnly` | Medium | Same protection class as above for the delete path. |
| `TestCreate` | Medium | Basic settings CRUD happy path — settings underpin much of Rancher's runtime configuration. |
| `TestCreateExisting` | Low | Conflict-handling edge case. |
| `TestUpdate` | Medium | Same basic CRUD importance as `TestCreate`, update path. |
| `TestUpdateNonExisting` | Low | Simple 404 edge case. |
| `TestUpdateLink` | Low | UI affordance detail (link visibility) — cosmetic, not functional. |

## steveapi/

Steve is the primary API the modern Dashboard is built on — broad blast radius if broken.

| Test | Tier | Why |
|---|---|---|
| `TestExtensionAPIServer` (Local) | High | Foundational discovery/schema endpoints for any UI extension relying on the extension API server. |
| `TestExtensionAPIServerAuthorization` | High | Broad authorization-boundary check (metrics/health/etc. correctly locked down) — unauthenticated metrics/health leakage is a known vulnerability class. |
| `TestExtensionAPIServerCreateRequests` | Medium | Functional CRUD happy-path for two specific extension resource types. |
| `TestExtensionAPIServerUpdateRequests` | Medium | Same class as above, update path. |
| `TestExtensionAPIServerDeleteRequests` | Medium | Same class as above, delete path. |
| `TestExtensionAPIServer` (Downstream) | Medium | Confirms the extension API is correctly *not* exposed downstream — a scoping boundary, though the failure mode leaks metadata rather than enabling compromise. |
| `TestList` | Critical | 100+ cases validating Steve's list/filter/sort/paginate correctly enforce per-user RBAC scoping — a regression could leak resources (including secrets) to unauthorized users, and it's the primary mechanism the whole Dashboard uses to list anything. |
| `TestLinks` | Low | Hyperlink metadata format on returned objects — a HATEOAS convenience detail. |
| `TestCRUD` | High | Basic Steve CRUD — the mechanism nearly the entire Dashboard relies on for every resource type. |

## tokens/

Tokens are the backbone of session/authentication.

| Test | Tier | Why |
|---|---|---|
| `TestCurrentToken` | Medium | "Which token is mine" bookkeeping — useful but smaller blast radius than authentication itself. |
| `TestWebsocket` | Medium-High | Rejects a specific request-smuggling-adjacent vector (websocket upgrade to a protected endpoint) — narrow but security-relevant. |
| `TestAPITokenTTL` | High | Confirms tokens can't bypass the configured max TTL — undermines an admin security policy install-wide if broken. |
| `TestKubeconfigTokenTTL` | High | Confirms kubeconfig token expiry works end-to-end across two API surfaces — core session security with broad reach (anyone downloading a kubeconfig). |

## users/

| Test | Tier | Why |
|---|---|---|
| `TestUserCantDeleteSelf` | Medium-High | Prevents an admin from locking themselves (or everyone, if the last admin) out. |
| `TestUserCantDeactivateSelf` | Medium-High | Same risk class as above via deactivation instead of deletion. |
| `TestUserCantUseUsernameAsPassword` | Medium | Real password-policy control, though one specific weak-password pattern. |
| `TestPasswordTooShort` | Medium | Same password-policy class as above, minimum-length rule. |

## workloads/

### dns_test.go

| Test | Tier | Why |
|---|---|---|
| `TestDNSFields` | Low | Schema field-permission metadata — implementation detail. |
| `TestDNSHostname` | Medium | Functional CRUD happy path for hostname-based DNS records — a real, used legacy feature. |
| `TestDNSIPs` | Medium | Same CRUD importance as above for IP-based records, plus a bundled security-relevant validation (rejecting loopback IPs). |

### ingress_test.go

| Test | Tier | Why |
|---|---|---|
| `TestIngressFields` | Low | Schema field-permission metadata check. |
| `TestIngress` | High | Basic ingress creation is a fundamental, extremely common workload-exposure action. |
| `TestIngressRulesSameHostPortPath` | Medium | Narrower correctness detail (rule-merging logic) on the same capability as above. |

### namespaced_secrets_test.go
CRUD across 5 secret kinds at namespace scope — mostly repetitive coverage of the same mechanism.

| Test | Tier | Why |
|---|---|---|
| `TestNamespacedSecrets` | Medium | Establishes the core CRUD pattern for the most common secret kind (Opaque). |
| `TestNamespacedCertificates` | Low-Medium | Same CRUD pattern, different secret kind — largely redundant mechanism. |
| `TestNamespacedDockerCredential` | Low-Medium | Same pattern; adds a real write-only-field security check (password not returned). |
| `TestNamespacedBasicAuth` | Low-Medium | Same pattern; same write-only-field check as above, repeated for a different kind. |
| `TestNamespacedSSHAuth` | Low-Medium | Same pattern; same write-only-field check, repeated again. |

### secrets_test.go
Project-scoped mirror of `namespaced_secrets_test.go` — same redundancy, plus two distinct kubectl-interop tests.

| Test | Tier | Why |
|---|---|---|
| `TestSecrets` | Medium | Core CRUD pattern at project scope for Opaque secrets. |
| `TestCertificates` | Low-Medium | Same pattern, certificate kind. |
| `TestDockerCredential` | Low-Medium | Same pattern, docker-credential kind (write-only check repeated). |
| `TestBasicAuth` | Low-Medium | Same pattern, basic-auth kind (write-only check repeated). |
| `TestSSHAuth` | Low-Medium | Same pattern, SSH kind (write-only check repeated). |
| `TestSecretCreationKubectl` | Medium-High | Distinct and valuable: confirms kubectl-created secrets are correctly surfaced through Rancher's API — many users don't create secrets through Rancher itself. |
| `TestMalformedSecretParse` | Medium | Real resilience check — one malformed certificate shouldn't break the whole secrets list for a project. |

### workload_test.go
The most functionally rich file in `workloads/` — core Deployment/StatefulSet lifecycle via Norman.

| Test | Tier | Why |
|---|---|---|
| `TestDeploymentCreationKubectl` | Medium-High | Confirms kubectl-created deployments surface through Rancher's Norman API — same interop value class as the kubectl secret test. |
| `TestWorkloadPortKinds` | Medium | Confirms all 4 service-exposure types are handled — broadly used, though essentially one parameterized check across 4 values. |
| `TestWorkloadImageChangePrivateRegistry` | Medium-High | Private-registry pull-secret auto-selection is a common real-world pain point; regressions break deployments silently. |
| `TestWorkloadPortsChange` | Medium | Confirms ClusterIP is correctly assigned/cleared as ports change — real but narrower networking-correctness detail. |
| `TestWorkloadProbes` | Medium | Probes are widely used and important for workload health, but this only checks field persistence through Rancher's API, not that Kubernetes acts on them. |
| `TestWorkloadScheduling` | Low-Medium | Narrow field-persistence check for one scheduler-name field. |
| `TestStatefulSetWorkloadVolumeMountSubpath` | Medium | Real security-relevant validation (path-traversal / absolute-path rejection), scoped narrowly to StatefulSets. |
| `TestWorkloadRedeploy` | Medium | A common, holistic user action (forcing a rolling restart). |
| `TestWorkloadActionReadOnly` | Medium-High | Real RBAC boundary — read-only users can't trigger workload actions like rollback — same class as the other RBAC read-only tests. |
| `TestHPA` | Medium | HPA creation with multiple metric types is a commonly-used autoscaling feature, though this only checks creation/initial state, not actual scaling behavior. |

---

## Tier summary

| Tier | Count |
|---|---|
| Critical | 7 |
| High | 33 |
| Medium-High | 16 |
| Medium | 56 |
| Low-Medium | 24 |
| Low | 23 |
| Irrelevant | 0 |
| **Total** | **159** |

**Critical tests** (the 7 where a regression is closest to "Rancher stops functioning" or "a
privilege boundary fails"): `TestInstallWebhook`, `TestK8sProxyFetchesNamespacesFromLocalCluster`,
`TestK8sProxyFetchesNamespacesFromDownstreamCluster`, `TestClusterCreateDefaultRole`,
`TestProjectCreateDefaultRole`, `TestOnlyAdminCanCRUDGlobalRoles`, `TestList` (steveapi).
