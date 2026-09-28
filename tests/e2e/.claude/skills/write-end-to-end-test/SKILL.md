---
name: write-end-to-end-test
description: Turn a plain-English description of test steps into a new Go end-to-end test added to an existing suite under tests/e2e, following this suite's shepherd-session conventions. Use when asked to write, add, or create an end-to-end/e2e test, or to turn a description of test steps into a test under tests/e2e.
---

# Write End-to-End Test

Add **one new test** to an existing suite under `tests/e2e`, from a plain-English description of
what it should do. This skill adds a single test method to the best-fitting existing suite — it
does not restructure suites, split files, or fix existing tests. If the input seems to call for
more than one new test, or for changes to existing tests, stop and ask before doing either.

Ambiguity is resolved by **asking**, never by assuming — this applies throughout, but especially
to cluster targeting (see Step 2 below), since guessing wrong there produces a test that silently
checks the wrong thing.

## Input format

The input looks exactly like a summary entry, given directly in the prompt rather than read from a
file — a one-sentence flow plus a list of checks:

```
<one-sentence flow: setup + primary action, present tense>
- <check 1>
- <check 2>
```

Example:

```
Bind a restricted user (global role "user-base") to the "backups-manage" ClusterRoleTemplate
on the local cluster.
- Checks the user can list etcdbackups in the local cluster's namespace once RBAC propagates.
```

There is no required schema beyond this — no labeled "Test name:"/"Cluster:"/"Suite:" fields. The
workflow below extracts the test name, cluster targeting, and suite placement from the prose, and
asks whenever any of those aren't unambiguous.

## Conventions

These are the rules a generated test must follow. They are policy, not just a description of what
exists today — see the note at the end of this section for where the current code falls short of
them.

### Suite structure

Each suite lives under `tests/e2e/<package>/` as a `testify/suite.Suite`. One dedicated
`<name>_suite_test.go` holds everything the suite's tests share; every other file in the directory
only adds test methods to that same struct — no setup, no shared assertions.

```go
type RTBTestSuite struct {
	suite.Suite
	client              *rancher.Client
	session             *session.Session
	project             *management.Project // suite-shared fixture
	downstreamClusterID string
}

func (p *RTBTestSuite) SetupSuite() {
	p.downstreamClusterID = "local"
	testSession := session.NewSession()
	p.session = testSession

	client, err := rancher.NewClient("", testSession)
	p.Require().NoError(err)
	p.client = client

	testProject, err := client.Management.Project.Create(&management.Project{
		ClusterID: p.downstreamClusterID,
		Name:      "TestProject",
	})
	p.Require().NoError(err)
	p.project = testProject
}

func (p *RTBTestSuite) TearDownSuite() {
	client, err := p.client.WithSession(p.session)
	p.Require().NoError(err)
	p.Require().NoError(client.Management.Project.Delete(p.project))
	p.session.Cleanup()
}

func TestRTBTestSuite(t *testing.T) {
	suite.Run(t, new(RTBTestSuite))
}
```

Only resources meant to be shared across *every* test in the suite belong in `SetupSuite`. They're
the one thing that needs manual deletion, in `TearDownSuite`, because they weren't created through
a sub-session.

A suite is not always confined to the file that declares its struct: `rbac/` is one suite
(`RTBTestSuite`, declared in `rtbs_test.go`) with its test methods spread across seven files by
topic (`default_roles_test.go`, `etcdbackups_test.go`, `features_test.go`, `global_roles_test.go`,
`impersonation_test.go`, `projects_test.go`, `rtbs_test.go`). Most other directories are simpler —
one file, one suite. Either is valid; check for sibling files adding methods to the same struct
before assuming a directory's suite is confined to one file.

A related but distinct pattern is a shared base struct via embedding — see
`steveapi/steve_api_test.go`, where an unexported `steveAPITestSuite` holds common fields/helpers
and `LocalSteveAPITestSuite`/`DownstreamSteveAPITestSuite` each embed it and define their own
`SetupSuite`. Reach for this only when a new test genuinely needs a different environment/setup
(e.g. a local-cluster variant vs. a downstream-cluster variant of the same suite), not as an
alternative to topic-file splitting.

This pattern has a non-obvious consequence worth knowing before generating a test in this shape: a
test method defined directly on the *shared base* (not on either concrete embedding type) runs once
per concrete suite that embeds it — e.g. `steveAPITestSuite.TestLinks` executes both when
`TestSteveLocal` runs (against the local cluster) and again when `TestSteveDownstream` runs (against
a real downstream cluster), with no duplicated code. This is a structural fact about the test, not a
behavioral one, so a plain-English description of "what the test does" will never mention it — if a
new test's behavior isn't inherently local-only or downstream-only, and it's going into an area that
already has this local/downstream split, ask explicitly whether it should run once or against both
contexts (see Step 2), rather than assuming "one cluster" just because the input only describes one
run of it.

When a test does need to target "a" downstream cluster and the input doesn't name one, prefer
resolving it the way most of this codebase already does — `client.RancherConfig.ClusterName` (the
cluster configured in `config.yaml`) — over inventing new cluster-discovery logic. Polling for any
active/Ready cluster (as `clusters/k8s_proxy_test.go` does) is a real but less common pattern; ask
if it's genuinely unclear which of the two the input wants, rather than defaulting to the discovery
approach.

### Per-test isolation and idempotent cleanup

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

### Indirectly-created resources need explicit cleanup

Sub-session auto-cleanup only tracks objects created by a direct call on the session-scoped client
(`client.Management.X.Create(...)`). If a resource is instead created as a *side effect* — a
Rancher controller reacting to something the test did, rather than the test's own client call — the
session never sees it, and it leaks unless the test registers cleanup manually.

```go
// Creating a CRTB with a principal that doesn't match any existing user triggers the CRTB
// controller to create a new User via EnsureUser. The test's client never calls
// client.Management.User.Create() directly, so the session doesn't know this User exists —
// it must be cleaned up by hand.
crtb, err := client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
	ClusterID:       "local",
	RoleTemplateID:  "cluster-owner",
	UserPrincipalID: fakePrincipal,
})
p.Require().NoError(err)

user, err := client.Management.User.ByID(crtb.UserID) // populated once the controller reacts
p.Require().NoError(err)
p.T().Cleanup(func() { _ = client.Management.User.Delete(user) })
```

Recognize this pattern whenever the input describes an action that *causes* something to be
created rather than creating it directly (e.g. "triggers new-user creation," "the controller
provisions X") — and add the manual cleanup, don't rely on the sub-session for it.

### Exact counts against global default-flag mechanisms require clearing state first

Some resources carry a boolean "this is a default" flag that affects *every* future instance of a
parent action, cluster-wide — e.g. `RoleTemplate.ClusterCreatorDefault`/`.ProjectCreatorDefault`,
`GlobalRole.NewUserDefault`. Rancher ships several role templates/global roles with these flags
already set. A literal "exactly N" assertion against a newly created cluster/project/user is
contaminated by those pre-existing defaults unless they're cleared first and restored afterward:

```go
// Clears every role template's existing ClusterCreatorDefault flag, sets it only on roleIDs,
// and registers a cleanup that restores the original flags. This is what makes a later
// `Require().Len(crtbs.Data, 3)` a true "exactly 3, no more" assertion rather than an
// undercount-or-overcount depending on whatever else is currently flagged as default.
func (p *RTBTestSuite) setClusterCreatorDefaults(client *rancher.Client, roleIDs []string) {
	roleTemplates, err := client.Management.RoleTemplate.List(nil)
	p.Require().NoError(err)

	originals := map[string]bool{}
	for _, rt := range roleTemplates.Data {
		if rt.ClusterCreatorDefault {
			originals[rt.ID] = true
		}
	}
	for i := range roleTemplates.Data {
		rt := &roleTemplates.Data[i]
		if rt.ClusterCreatorDefault {
			_, err := client.Management.RoleTemplate.Update(rt, map[string]any{"clusterCreatorDefault": false})
			p.Require().NoError(err)
		}
	}
	for _, id := range roleIDs {
		rt, err := client.Management.RoleTemplate.ByID(id)
		p.Require().NoError(err)
		_, err = client.Management.RoleTemplate.Update(rt, map[string]any{"clusterCreatorDefault": true})
		p.Require().NoError(err)
	}

	p.T().Cleanup(func() {
		// restore every role template's original flag value here
	})
}
```

This is a cleanup category distinct from deleting a created object: **restoring mutated state on
objects the test didn't create.** It applies to any global/shared toggle, not just these three
fields. If the input just says "exactly N" without saying whether other defaults need clearing,
that's not resolvable from the text alone — ask (see Step 2 of the Workflow).

### Assertions always live in the test body

Helper functions — suite-level or test-local — may only use `Require()`/`Assert()` as a **fail-fast
guard on their own setup action**, never to express the behavior the test is actually proving.

```go
// OK: guards that setup succeeded. Not a behavioral assertion.
func (p *RTBTestSuite) createUser(client *rancher.Client, prefix, globalRole string) *management.User {
	user, err := users.CreateUserWithRole(client, &management.User{...}, globalRole)
	p.Require().NoError(err) // guard clause on setup — allowed
	return user
}
```

```go
// NOT a valid pattern to add, even though a version of it exists in rtbs_test.go today:
func (p *RTBTestSuite) assertClusterAccessRevoked(userClient *rancher.Client) {
	p.Require().Eventually(func() bool { ... }, ...) // asserts the actual
	_, err := userClient.Management.Cluster.ByID(p.downstreamClusterID)      // behavior under test —
	p.Require().Error(err)                                                  // must be inline instead.
}
```

If two tests need an identical, lengthy check, it gets duplicated inline in both. That's an accepted
DRY violation, not an oversight.

### Three tiers of code reuse

1. **Inline** — a one-off action, written directly in the test.
2. **Test-local closure** — defined *inside* one test function, for repetition local to that test
   only (e.g. a helper closure used twice within a single test and nowhere else).
3. **Suite-level helper** — setup/action (never assertion) shared across multiple tests in the
   suite (e.g. `createUser`, `createNamespace`), defined in the suite's setup file.

Before writing a new suite-level helper, check whether an equivalent already exists under
`tests/e2e/actions/` (e.g. `tests/e2e/actions/kubeapi/namespaces`, `.../kubeapi/rbac`,
`.../kubeapi/secrets`) — reuse it rather than duplicating it at the suite level.

### Current known gaps (do not fix as part of this skill)

`rtbs_test.go` — the reference file above — doesn't fully match this policy today: it mixes its own
nine tests into the suite's setup file, and it still has the `assertClusterAccessRevoked` helper
shown above as a counter-example. Both are tracked for a later, separate audit pass. Adding a new
test does not require or invite fixing either — leave them as-is.

## Workflow

### Step 1 — Parse the input

Read the lead sentence and bullet checks. Identify the setup/actions, the checks, and anything
already stated about cluster targeting.

### Step 2 — Resolve ambiguity (always ask, never assume)

For every resource creation, mutation, or check in the input:

- **Management-plane objects** (`RoleTemplate`, `GlobalRole`, `GlobalRoleBinding`, `Project`,
  `User`, `Setting`, `Feature`, `EtcdBackup`, CRTB/PRTB) — no question about *where the object
  itself lives* (always the management API), but if it references a `ClusterID`/`ProjectID` and the
  input doesn't say which cluster, ask.
- **Live cluster-scoped objects or access checks** (`Namespace`, `Secret`, `Node`, `Pod`, `PVC`,
  `Ingress`, `Workload`, `ConfigMap`, any can-I-do-X check) — ask which cluster every time it isn't
  explicit for *that specific step*, including whether it's the same cluster as a preceding step in
  the same test. Never assume continuity between steps.
- **An "exactly N" check against a resource driven by a global default-flag mechanism** (see the
  Conventions section) — ask whether pre-existing defaults elsewhere in the system need to be
  cleared (and restored afterward) for the count to hold, rather than assuming a freshly created
  object is the only contributor.
- **A generic/cluster-agnostic behavior going into an area that already has a local/downstream
  embedding split** (see "Suite structure" in Conventions) — ask whether the new test should run
  once against a single cluster, or against both contexts like its neighbors, rather than assuming
  "one cluster" just because the input only describes one run of it.

Also confirm the proposed Go test function name (inferred from the lead sentence, `TestXxx`
PascalCase), and flag anything else vague enough that two engineers would reasonably write
different code from it (e.g. the input says "should fail" but not which status code or error text).

### Step 3 — Find the best-fit home

- Match against existing suites by fixture overlap: does a suite's `SetupSuite` already build what
  this test needs (same client scope, same shared project/cluster fixture, same resource types
  exercised nearby)?
- State the candidate suite and file, and *why*, and confirm before proceeding — even when
  confident.
- If the suite fits but no existing file in it is a topical match: ask — add to the closest file
  anyway, or start a new topic file in that same suite?
- If no existing suite fits at all: ask whether to create a new suite (new file, possibly new
  directory). Never do this silently.

### Step 4 — Identify reusable code

- Read the target suite's setup file for what already exists (the sub-session helper, shared
  fixtures, suite-level helpers like `createUser`) and reuse it.
- Check `tests/e2e/actions/` for existing helpers covering the resource types involved before
  writing anything new.
- Only add a new suite-level helper if nothing existing (suite-level or in `actions/`) covers a
  repeated setup need — call this out explicitly, since it's the one case where adding "one test"
  still touches the shared setup file.

### Step 5 — Generate

- Write exactly one new test method on the target suite type, in the target file.
- First line: obtain a client via the suite's sub-session helper.
- Every behavioral assertion inline, per the Conventions section — never factored into a shared
  helper.
- Cluster targeting exactly as resolved in Step 2 for each resource/check.
- If the input describes an action that causes a resource to be created indirectly (a controller
  reacting to something the test did, not the test's own `.Create()` call), add an explicit
  `T().Cleanup` for it — see "Indirectly-created resources need explicit cleanup" in Conventions.
  Don't rely on the sub-session to catch it.

### Step 6 — Validate

- Run `go build` and `go vet` on the affected package.
- Surface any compile failure rather than silently reworking it — report it and either fix the
  specific issue or hand it back with the error.

### Step 7 — Hand back

- Present the generated method (and any new helper from Step 4) for review before considering the
  task done.
