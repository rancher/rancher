---
name: write-e2e-test
description: Turn a plain-English description of test steps into a new Go end-to-end test added to an existing suite under tests/e2e, following this suite's shepherd-session conventions. Use when asked to write, add, or create an end-to-end/e2e test, or to turn a description of test steps into a test under tests/e2e.
---

# Write E2E Test

Add **one new test** to an existing suite under `tests/e2e` — or, when the input describes a
multi-stage progression that turns out to split cleanly (see Step 2), one test per stage — from a
plain-English description of what it should do. This skill adds test methods to the best-fitting
existing suite — it does not restructure suites, split existing files, or fix existing tests. If
the input describes genuinely unrelated behaviors bundled together (not a splittable progression),
stop and ask before generating anything.

Ambiguity is resolved by **asking**, never by assuming — this applies throughout, but especially
to cluster targeting (see Step 3 below), since guessing wrong there produces a test that silently
checks the wrong thing.

The rules a generated test must follow are in [conventions.md](conventions.md). Read it at Step 4,
before choosing where the test goes.

## Input format

The input looks exactly like a summary entry produced by the `summarize-e2e-test` skill, given
directly in the prompt rather than read from a file — **Arrange** bullets, a single **Act**
sentence, and **Assert** bullets:

```
**Arrange:**
- Create a restricted user with global role "user-base".

**Act:** Bind the user to the "backups-manage" ClusterRoleTemplate on the local cluster.

**Assert:**
- Checks the user can list etcdbackups in the local cluster's namespace once RBAC propagates.
```

There is no required schema beyond these labels — no separate "Test name:"/"Cluster:"/"Suite:"
fields. The workflow below extracts the test name, cluster targeting, and suite placement from the
Arrange/Act text, and asks whenever any of those aren't unambiguous.

In practice the input often arrives as plain prose instead ("create a namespace and a global role,
assign it to a user, check the user can access..."). That's fine — Step 1 restates it in this format
before anything else happens.

Each Arrange bullet maps to one setup step in the generated test body; Act maps to the single call
being tested; each Assert bullet maps to one inline assertion following it. **Arrange** may be
absent entirely — that means the input describes a test with no precondition beyond the Act itself
(e.g. calling `Create` directly with inline field values meant to trigger a validation error).
Don't invent a setup step to fill it in.

The input may instead use `summarize-e2e-test`'s multi-stage format: one shared **Arrange**
followed by numbered **Act 1:**/**Assert 1:**, **Act 2:**/**Assert 2:**, ... pairs, where each
stage builds on the state left by the previous one. This does **not** automatically become one
multi-stage test method — see Step 2.

## Workflow

### Step 1 — Parse the input

Read the Arrange bullets, the Act sentence, and the Assert bullets. Identify the setup/actions, the
checks, and anything already stated about cluster targeting.

If the input is prose rather than Arrange/Act/Assert, restate it in that format first and show the
restatement back as part of the Step 3 confirmation. Writing it out this way is what exposes the
ambiguities Step 3 looks for — which step is the single Act, which cluster each step targets, and
which checks are vague.

### Step 2 — Multi-stage input: decide whether to split

Skip this step entirely for single-Act input. If the input has numbered `Act N`/`Assert N` pairs,
decide whether to generate several independent single-stage tests or one multi-stage test method:

- Check whether each stage could stand alone: if stage N's Arrange would just be the shared Arrange
  plus stage N-1's Act — with no dependency on stage N-1's *assertion* itself beyond the state it
  left behind — it can be split into its own test.
- **Default to splitting** into N separate single-stage tests, each with its own Arrange (the
  shared bullets plus whichever earlier Acts are needed to reach that stage's starting state), one
  Act, one Assert. Simple, single-stage tests are the goal going forward — multi-stage should be
  the exception, not the default output shape.
- Only generate a single multi-stage test method when splitting would genuinely lose coverage —
  e.g. the thing being tested is the *transition itself* (that a controller correctly reacts to a
  second change on top of a first, in the same run), not just two facts that happen to be checked
  in sequence.
- State which you're doing and why, and confirm before proceeding — even when confident. If it's
  genuinely unclear which applies, ask rather than defaulting silently.
- This decision determines how many test methods Step 6 generates, and feeds into Step 4's
  suite-fit check (a split test's fixture needs may differ per stage).

### Step 3 — Resolve ambiguity (always ask, never assume)

For every resource creation, mutation, or check in the input:

- **Management-plane objects** (`RoleTemplate`, `GlobalRole`, `GlobalRoleBinding`, `Project`,
  `User`, `Setting`, `Feature`, `EtcdBackup`, CRTB/PRTB) — no question about *where the object
  itself lives* (always the management API), but if it references a `ClusterID`/`ProjectID` and the
  input doesn't say which cluster, ask.
- **Live cluster-scoped objects or access checks** (`Namespace`, `Secret`, `Node`, `Pod`, `PVC`,
  `Ingress`, `Workload`, `ConfigMap`, any can-I-do-X check) — ask which cluster every time it isn't
  explicit for *that specific step*, including whether it's the same cluster as a preceding step in
  the same test. Never assume continuity between steps.
- **An "exactly N" check against a resource driven by a global default flag**
  (`RoleTemplate.ClusterCreatorDefault`/`.ProjectCreatorDefault`, `GlobalRole.NewUserDefault`, or
  any similar cluster-wide toggle) — ask whether pre-existing defaults need to be cleared (and
  restored afterward) for the count to hold, rather than assuming a freshly created object is the
  only contributor.
- **A generic/cluster-agnostic behavior going into an area that already has a local/downstream
  embedding split** (e.g. `tests/e2e/steveapi/`) — ask whether the new test should run once against
  a single cluster, or against both contexts like its neighbors, rather than assuming "one cluster"
  just because the input only describes one run of it.
- **A permission-grant check** ("the user can access X", "the role grants Y") — the input almost
  never pins down everything that determines what the test proves. Ask about each of these that
  isn't explicit:
  - **Resource and verbs** — "access the resources" could mean get on secrets, list on configmaps,
    or `*` on everything.
  - **The user's base global role** — `user-base` isolates the grant under test; `user` brings
    extra permissions that can mask or muddy it.
  - **Forbidden-before-grant check** — a 403 before the Act, proving the Act is what grants access.
  - **Forbidden-outside-scope check** — a 403 on the same resource just outside the granted scope
    (another namespace, project, or cluster), proving the grant doesn't leak.

  Offer the two negative checks as a question; never add them silently. They make the positive
  check meaningful, but they're still checks the input didn't ask for.

Also confirm the proposed Go test function name(s) (inferred from the Act sentence — one name per
generated test if Step 2 split the input, `TestXxx` PascalCase), and flag anything else vague
enough that two engineers would reasonably write different code from it (e.g. the input says
"should fail" but not which status code or error text).

### Step 4 — Find the best-fit home

Read [conventions.md](conventions.md) now if you haven't already.

- Match against existing suites by fixture overlap: does a suite's `SetupSuite` already build what
  this test needs (same client scope, same shared project/cluster fixture, same resource types
  exercised nearby)?
- State the candidate suite and file, and *why*, and confirm before proceeding — even when
  confident.
- If the suite fits but no existing file in it is a topical match: ask — add to the closest file
  anyway, or start a new topic file in that same suite?
- If no existing suite fits at all: ask whether to create a new suite (new file, possibly new
  directory). Never do this silently.

### Step 5 — Identify reusable code

- Read the target suite's setup file for what already exists (the sub-session helper, shared
  fixtures, suite-level helpers like `createUser`) and reuse it.
- Check `tests/e2e/actions/` for existing helpers covering the resource types involved before
  writing anything new, and read the body of any you plan to call (see "Three tiers of code reuse"
  in conventions.md). If one rules out what the test needs, say so when confirming placement rather
  than working around it silently.
- Only add a new suite-level helper if nothing existing (suite-level or in `actions/`) covers a
  repeated setup need — call this out explicitly, since it's the one case where adding "one test"
  still touches the shared setup file.

### Step 6 — Generate

- Write one new test method per Step 2's decision: exactly one if the input was single-Act (or a
  multi-stage input kept as one test), or one per stage if Step 2 chose to split — all on the target
  suite type, in the target file.
- Follow conventions.md throughout — in particular: a sub-session client on the first line, every
  behavioral assertion inline, explicit cleanup for indirectly-created resources, and the RBAC
  propagation waits.
- Cluster targeting exactly as resolved in Step 3 for each resource/check.

### Step 7 — Validate

- From the repo root, run `go vet ./tests/e2e/<pkg>/ && go test -c -o /dev/null ./tests/e2e/<pkg>/`
  on the affected package. Don't use `go build` — e2e packages contain only `_test.go` files, so it
  fails with "no non-test Go files" without compiling anything.
- Surface any compile failure rather than silently reworking it — report it and either fix the
  specific issue or hand it back with the error.

### Step 8 — Hand back

- Present the generated method(s) (and any new helper from Step 5) for review before considering
  the task done.
