---
name: summarize-e2e-test
description: Analyze a Go end-to-end test file under tests/e2e and produce a concise, high-level markdown summary that describes what each test does and checks. Use when asked to summarize an e2e test, create a test summary, or document what a test file covers.
---

# Summarize E2E Test

Produce a concise, high-level summary of a Go end-to-end test file. The goal is a
"this test does X, then Y, then checks Z" description of each test — not a line-by-line
walkthrough.

The bar to aim for: **given only the summary (not the code), a reader could recreate a
test that exercises the same functionality.** Not line-for-line, but hitting the same
setup, the same actions, and the same behavioral assertions.

## Workflow

### Step 1: Analyze the file

Read the test file and identify:
- The overall theme of the file (what feature/area it covers).
- Each individual test function.
- For each test: the setup/preconditions, the actions it performs, and the outcomes it
  verifies.

Ignore helper functions, test-harness plumbing, and cleanup logic when writing the
summary — but DO fold what a helper accomplishes into the test's steps (e.g. "sets 3
RoleTemplates as cluster-creator defaults" rather than naming the helper).

### Step 2: Write the summary

Create a markdown file named `<test_file_name>_summary.md` (e.g.
`default_roles_test_summary.md`). Use the structure below.

## Format

```markdown
# `<file_name>.go` Summary

<a single sentence describing what the whole file verifies>

## `<TestName>`
<One sentence describing the setup and primary action(s), phrased as "does X, then Y">
- Checks <observable outcome / assertion>.
- Checks <observable outcome / assertion>.
```

### Rules for the content

- Start with **a single plain sentence** describing what the whole file verifies (no
  label or heading — just the sentence).
- **One section per test**, titled with the test function name in backticks.
- The lead sentence describes the **flow**: what is set up and what action triggers the
  behavior under test (e.g. "Sets 3 RoleTemplates as cluster-creator defaults, then
  creates a cluster.").
- The bullets are the **checks** — the observable outcomes the test asserts. Phrase them
  as behavior ("Checks only the 2 unlocked roles produce bindings"), not as API calls
  ("Require().Len(crtbs.Data, 2)").
- **Include the steps that create or mutate resources** (creating a CRTB, locking a role,
  setting a default) — these are needed to recreate the test. Do not omit them just
  because they are setup.
- **Include concrete values that matter** to reproducing the behavior: how many resources,
  which flags/labels, expected counts, expected role names. Avoid incidental details like
  timeouts, retry intervals, or randomly generated names.
- Keep each test to a **short lead sentence plus a few bullets**. If a test needs more than
  ~5 bullets, the summary is probably too granular.
- Do **not** document helper functions, imports, or cleanup as their own sections.

## Example

For a test that sets creator-default roles and verifies they get bound on cluster
creation:

```markdown
## `TestClusterCreateDefaultRole`
Sets 3 RoleTemplates as cluster-creator defaults, then creates a cluster.
- Checks the cluster reaches `InitialRolesPopulated`.
- Checks exactly 3 CRTBs are created, one per default role.
- Checks each CRTB is bound to a real user whose principal matches.
```

For a test verifying a locked default role is skipped:

```markdown
## `TestClusterCreateRoleLocked`
Sets 3 cluster-creator defaults, locks one of them, then creates a cluster.
- Checks the cluster reaches `InitialRolesPopulated`.
- Checks only the 2 unlocked roles produce CRTBs (locked role is skipped, others still bound).
```
