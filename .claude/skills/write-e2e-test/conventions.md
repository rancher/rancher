# E2E Test Conventions

These are the rules a generated test must follow. They are policy, not just a description of what
exists today — see "Current known gaps" at the end for where the current code falls short of them.

## Suite structure

Each suite lives under `tests/e2e/<package>/` as a `testify/suite.Suite`. One dedicated
`<name>_suite_test.go` holds everything the suite's tests share; every other file in the directory
only adds test methods to that same struct — no setup, no shared assertions.

For the shape of a suite, read `RTBTestSuite` in `tests/e2e/rbac/rtbs_test.go`: the struct (client,
session, a suite-shared `project` fixture, `downstreamClusterID`), `SetupSuite`, `TearDownSuite`,
and the `TestRTBTestSuite` entry point.

Only resources meant to be shared across *every* test in the suite belong in `SetupSuite`. They're
the one thing that needs manual deletion, in `TearDownSuite`, because they weren't created through
a sub-session.

A suite is not always confined to the file that declares its struct: `tests/e2e/rbac/` is one suite
(`RTBTestSuite`, declared in `rtbs_test.go`) with its test methods spread across seven files by
topic (`default_roles_test.go`, `etcdbackups_test.go`, `features_test.go`, `global_roles_test.go`,
`impersonation_test.go`, `projects_test.go`, `rtbs_test.go`). Most other directories are simpler —
one file, one suite. Either is valid; check for sibling files adding methods to the same struct
before assuming a directory's suite is confined to one file.

A related but distinct pattern is a shared base struct via embedding — see
`tests/e2e/steveapi/steve_api_test.go`, where an unexported `steveAPITestSuite` holds common
fields/helpers and `LocalSteveAPITestSuite`/`DownstreamSteveAPITestSuite` each embed it and define
their own `SetupSuite`. Reach for this only when a new test genuinely needs a different
environment/setup (e.g. a local-cluster variant vs. a downstream-cluster variant of the same suite),
not as an alternative to topic-file splitting.

This pattern has a non-obvious consequence: a test method defined directly on the *shared base*
(not on either concrete embedding type) runs once per concrete suite that embeds it — e.g.
`steveAPITestSuite.TestLinks` executes both when `TestSteveLocal` runs (against the local cluster)
and again when `TestSteveDownstream` runs (against a real downstream cluster), with no duplicated
code. This is a structural fact about the test, not a behavioral one, so a plain-English
description of "what the test does" will never mention it. That's why Step 3 asks about it.

When a test does need to target "a" downstream cluster and the input doesn't name one, prefer
resolving it the way most of this codebase already does — `client.RancherConfig.ClusterName` (the
cluster configured in `config.yaml`) — over inventing new cluster-discovery logic. Polling for any
active/Ready cluster (as `tests/e2e/clusters/k8s_proxy_test.go` does) is a real but less common
pattern; ask if it's genuinely unclear which of the two the input wants, rather than defaulting to
the discovery approach.

## Per-test isolation and idempotent cleanup

Every test's first line gets its own sub-session-scoped client, registering that session's cleanup
with `t.Cleanup` immediately. This is what makes cleanup automatic and makes tests safe to re-run:

```go
func (p *RTBTestSuite) newSubSession() *rancher.Client {
	subSession := p.session.NewSession()
	client, err := p.client.WithSession(subSession)
	p.Require().NoError(err)
	p.T().Cleanup(subSession.Cleanup)
	return client
}

func (p *RTBTestSuite) TestBackupsManageRole() {
	client := p.newSubSession()
	// every resource created through `client` from here on is deleted
	// automatically when this test ends — no manual cleanup needed.
	...
}
```

A generated test must obtain its client via the suite's sub-session helper before creating
anything. Creating a resource through the raw suite-level client instead of a sub-session client is
a leak — never introduce a new instance of this.

## Indirectly-created resources need explicit cleanup

Sub-session auto-cleanup only tracks objects created by a direct call on the session-scoped client
(`client.Management.X.Create(...)`). If a resource is instead created as a *side effect* — a
Rancher controller reacting to something the test did, rather than the test's own client call — the
session never sees it, and it leaks unless the test registers cleanup manually with `T().Cleanup`.

For a real example, read `TestUserCreateDefaultRole` in `tests/e2e/rbac/default_roles_test.go`:
creating a CRTB with a principal that doesn't match any existing user makes the CRTB controller
create a new User via `EnsureUser`. The test never calls `client.Management.User.Create()`, so it
waits for the CRTB's `UserID` to be populated and registers the User's deletion by hand.

Recognize this pattern whenever the input describes an action that *causes* something to be
created rather than creating it directly (e.g. "triggers new-user creation," "the controller
provisions X") — and add the manual cleanup, don't rely on the sub-session for it.

## Exact counts against global default-flag mechanisms require clearing state first

Some resources carry a boolean "this is a default" flag that affects *every* future instance of a
parent action, cluster-wide — e.g. `RoleTemplate.ClusterCreatorDefault`/`.ProjectCreatorDefault`,
`GlobalRole.NewUserDefault`. Rancher ships several role templates/global roles with these flags
already set. A literal "exactly N" assertion against a newly created cluster/project/user is
contaminated by those pre-existing defaults unless they're cleared first and restored afterward.

For the pattern, read `setClusterCreatorDefaults` in `tests/e2e/rbac/default_roles_test.go`: it
records which role templates currently have the flag, clears it on all of them, sets it only on the
requested IDs, and registers a cleanup that restores the original flags. That's what makes a later
`Require().Len(crtbs.Data, 3)` a true "exactly 3, no more" assertion.

This is a cleanup category distinct from deleting a created object: **restoring mutated state on
objects the test didn't create.** It applies to any global/shared toggle, not just these three
fields.

## Assertions always live in the test body

Helper functions — suite-level or test-local — may only use `Require()`/`Assert()` as a **fail-fast
guard on their own setup action**, never to express the behavior the test is actually proving.

- **OK:** `createUser` in `tests/e2e/rbac/rtbs_test.go` — its `Require().NoError` only guards that
  the user was created.
- **Not a valid pattern to add:** `assertClusterAccessRevoked` in the same file — it polls and
  asserts that cluster access is gone, which is the behavior under test, so it must be inline
  instead. It exists today only as a known gap (see below).

If two tests need an identical, lengthy check, it gets duplicated inline in both. That's an accepted
DRY violation, not an oversight.

## Three tiers of code reuse

1. **Inline** — a one-off action, written directly in the test.
2. **Test-local closure** — defined *inside* one test function, for repetition local to that test
   only (e.g. a helper closure used twice within a single test and nowhere else).
3. **Suite-level helper** — setup/action (never assertion) shared across multiple tests in the
   suite (e.g. `createUser`, `createNamespace`), defined in the suite's setup file.

Before writing a new suite-level helper, check whether an equivalent already exists under
`tests/e2e/actions/` (e.g. `tests/e2e/actions/kubeapi/namespaces`, `.../kubeapi/rbac`,
`.../kubeapi/secrets`) — reuse it rather than duplicating it at the suite level.

Read a helper's body, not just its signature, before relying on it — some carry preconditions the
signature doesn't show. For example, `tests/e2e/actions/kubeapi/namespaces.CreateNamespace` accepts
an empty `projectName`, but then waits for a `<project>-namespaces-edit` ClusterRole that never
appears and hangs until the watch times out. It can only create namespaces *in a project*.

## Waiting on RBAC propagation

Bindings (GRBs, CRTBs, PRTBs) are reconciled into Kubernetes RBAC asynchronously, so an access check
made immediately after creating one is racy. Use the pattern the suite already uses for each case:

- **Access should now be allowed** — wait with `extauthz.WaitForAllowed` (imported as
  `extauthz "github.com/rancher/shepherd/extensions/kubeapi/authorization"`). It uses
  SelfSubjectAccessReviews, which any authenticated user can create, even one with only
  `user-base`. *Then* make the real API call and assert on it:

  ```go
  err = extauthz.WaitForAllowed(userClient, clusterID, []*authzv1.ResourceAttributes{
  	{Verb: "get", Resource: "secrets", Namespace: ns.Name},
  })
  p.Require().NoError(err)

  _, err = secrets.GetSecretByName(userClient, clusterID, ns.Name, secret.Name, metav1.GetOptions{})
  p.Require().NoError(err)
  ```

- **Access should now be revoked** (after deleting/changing a binding) — poll with
  `Require().EventuallyWithT` until the call is forbidden:

  ```go
  p.Require().EventuallyWithT(func(c *assert.CollectT) {
  	_, err := secrets.GetSecretByName(userClient, clusterID, ns.Name, secret.Name, metav1.GetOptions{})
  	assert.Truef(c, apierrors.IsForbidden(err), "expected forbidden, got: %v", err)
  }, 2*time.Minute, 2*time.Second, "waiting for access to be revoked")
  ```

- **Access is denied before anything was granted** — nothing to propagate, so check it directly
  with `apierrors.IsForbidden(err)`, no wait.

## Current known gaps (do not fix as part of this skill)

`tests/e2e/rbac/rtbs_test.go` — the reference suite above — doesn't fully match this policy today:
it mixes its own nine tests into the suite's setup file, and it still has the
`assertClusterAccessRevoked` helper described above as a counter-example. Both are tracked for a
later, separate audit pass. Adding a new test does not require or invite fixing either — leave them
as-is.
