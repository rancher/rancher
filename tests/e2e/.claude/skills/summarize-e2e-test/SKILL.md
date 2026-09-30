---
name: summarize-e2e-test
description: Analyze a Go end-to-end test file under tests/e2e and produce a concise, high-level markdown summary that describes what each test does and checks. Use when asked to summarize an e2e test, create a test summary, or document what a test file covers.
---

# Summarize E2E Test

Produce a concise, high-level summary of a Go end-to-end test file, structured around
**Arrange / Act / Assert**: what each test sets up, the one action it performs, and the
outcomes it checks. Not a line-by-line walkthrough.

The bar to aim for: **given only the summary (not the code), a reader could recreate a
test that exercises the same functionality.** Not line-for-line, but hitting the same
setup, the same action, and the same behavioral assertions.

## Workflow

### Step 1: Analyze the file

Read the test file and identify:
- The overall theme of the file (what feature/area it covers).
- Each individual test function.
- For each test, break its body into the three AAA phases:
  - **Arrange** — every setup/precondition step: resource creation, mutation, or state
    change that exists to make the Act step possible (e.g. creating a user, setting a
    default role, locking a role).
  - **Act** — the single call that triggers the behavior actually under test. If a
    test's flow doesn't reduce to one clear triggering call, identify whichever call is
    immediately followed by the test's meaningful assertions and treat everything else
    as Arrange.
  - **Assert** — the observable outcomes the test verifies, drawn from the assertions
    that follow the Act step.

Some tests perform more than one meaningful mutate-then-verify cycle that builds on the
previous one — e.g. removing one resource limit and checking the result, then removing
another and checking again. Treat these as **multi-stage**: one shared Arrange, followed
by a numbered Act/Assert pair per cycle (see Format below), rather than forcing multiple
unrelated triggering actions into a single Act sentence. Reserve this for genuine
progressions where each stage's action depends on the state left by the previous stage —
not for a test that merely calls multiple functions in a row.

Ignore helper functions, test-harness plumbing, and cleanup logic when writing the
summary — but DO fold what a helper accomplishes into the appropriate phase (e.g. "sets
3 RoleTemplates as cluster-creator defaults" as an Arrange bullet, rather than naming
the helper).

### Step 2: Write the summary

Create a markdown file named `<test_file_name>_summary.md` (e.g.
`default_roles_test_summary.md`). Use the structure below.

## Format

```markdown
# `<file_name>.go` Summary

<a single sentence describing what the whole file verifies>

## `<TestName>`
**Arrange:**
- <setup/precondition step>
- <setup/precondition step>

**Act:** <the single action that triggers the behavior under test, present tense>

**Assert:**
- <observable outcome / assertion>
- <observable outcome / assertion>
```

For a genuine multi-stage progression (see Step 1), number the Act/Assert pairs instead
of forcing them into one Act sentence:

```markdown
## `<TestName>`
**Arrange:**
- <setup/precondition step>

**Act 1:** <first action, present tense>
**Assert 1:**
- <observable outcome / assertion>

**Act 2:** <second action, present tense, building on the state left by Assert 1>
**Assert 2:**
- <observable outcome / assertion>
```

### Rules for the content

- Start with **a single plain sentence** describing what the whole file verifies (no
  label or heading — just the sentence).
- **One section per test**, titled with the test function name in backticks.
- **Arrange** lists every setup/precondition step as its own bullet — resource creation,
  mutation, or state change needed before the behavior under test can run (e.g. creating
  a CRTB, locking a role, setting a default). Include concrete values that matter to
  reproducing the behavior (how many resources, which flags/labels, expected role
  names); omit incidental details like timeouts, retry intervals, or randomly generated
  names. Even a single setup step still gets one bullet, for consistency. If a test has
  no precondition distinct from the Act itself (e.g. it directly calls `Create` with
  inline field values meant to trigger a validation error), omit the **Arrange** section
  entirely rather than inventing a step — don't describe constructing the request object
  as if it were setup.
- **Act is exactly one sentence** naming the one action that triggers the behavior under
  test (e.g. "Creates a cluster."). If a test's flow only makes sense with more than one
  triggering action, that's a sign the test may be exercising more than one behavior —
  summarize the primary trigger as Act rather than merging multiple actions into one
  sentence.
- Use **numbered Act/Assert pairs** (`Act 1`/`Assert 1`, `Act 2`/`Assert 2`, ...) only for
  a genuine progression, where a later stage's action depends on the state left by an
  earlier stage's assertion. Default to the single Act/Assert form; multi-stage should be
  the exception. Each numbered Act is still exactly one sentence.
- **Assert** lists the observable outcomes the test verifies, one per bullet. Phrase
  them as behavior ("Checks only the 2 unlocked roles produce bindings"), not as API
  calls ("Require().Len(crtbs.Data, 2)").
- Keep Arrange and Assert lists short — a handful of bullets each. If either needs more
  than ~5, the summary is probably too granular.
- Do **not** document helper functions, imports, or cleanup as their own sections.

## Example

For a test that sets creator-default roles and verifies they get bound on cluster
creation:

```markdown
## `TestClusterCreateDefaultRole`
**Arrange:**
- Sets 3 RoleTemplates as cluster-creator defaults.

**Act:** Creates a cluster.

**Assert:**
- Checks the cluster reaches `InitialRolesPopulated`.
- Checks exactly 3 CRTBs are created, one per default role.
- Checks each CRTB is bound to a real user whose principal matches.
```

For a test verifying a locked default role is skipped:

```markdown
## `TestClusterCreateRoleLocked`
**Arrange:**
- Sets 3 cluster-creator defaults.
- Locks one of them.

**Act:** Creates a cluster.

**Assert:**
- Checks the cluster reaches `InitialRolesPopulated`.
- Checks only the 2 unlocked roles produce CRTBs (locked role is skipped, others still
  bound).
```

For a test that progressively removes resource-quota limits and checks the namespace
quota after each removal — a genuine multi-stage progression:

```markdown
## `TestRemoveQuotaFromProjectWithNamespacePropagation`
**Arrange:**
- Creates a project with resource quota limits of 500m CPU and 10 ConfigMaps, and
  namespace-default limits of 200m CPU and 5 ConfigMaps.
- Creates a namespace in that project.

**Act 1:** Removes the CPU limit from the project and its namespace default.
**Assert 1:**
- Checks the namespace's resource quota retains just the ConfigMaps limit (5).

**Act 2:** Removes the ConfigMaps limit as well.
**Assert 2:**
- Checks the namespace's resource quota object is deleted entirely.
```
