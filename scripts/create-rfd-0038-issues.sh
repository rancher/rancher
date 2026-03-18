#!/usr/bin/env bash
# create-rfd-0038-issues.sh
#
# Creates GitHub issues for every work item in the RFD 0038
# "Day 2 Ops for Imported Clusters" planning CSV.
#
# Prerequisites:
#   * gh CLI authenticated with a token that has 'repo' scope:
#       gh auth login
#   * Run from inside the rancher/rancher repository clone, or pass
#     the repository explicitly via --repo.
#
# Usage:
#   ./scripts/create-rfd-0038-issues.sh [--repo owner/repo] [--dry-run]
#
# --dry-run   Print the gh commands without executing them.
# --repo      Override the target repository (default: rancher/rancher)

set -euo pipefail

REPO="rancher/rancher"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)    REPO="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

# ---------------------------------------------------------------------------
# Helper – create one issue and print its URL.
# Usage: create_issue <title> <body>
# ---------------------------------------------------------------------------
create_issue() {
  local title="$1"
  local body="$2"

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "=== DRY RUN: gh issue create ==="
    echo "  --repo  $REPO"
    echo "  --title $title"
    echo "  --body  (see below)"
    echo "$body"
    echo ""
    return
  fi

  local url
  url=$(gh issue create \
    --repo "$REPO" \
    --title "$title" \
    --body  "$body")
  echo "Created: $url"
}

# ---------------------------------------------------------------------------
# Shared preamble used in every issue body
# ---------------------------------------------------------------------------
RFD_LINK="https://github.com/rancher/rancher/issues/53153"

preamble() {
  cat <<EOF
> Part of **RFD 0038 – Day 2 Ops for Imported Clusters**.
> Tracking issue / RFD context: $RFD_LINK

EOF
}

size_legend() {
  cat <<'EOF'

---
**Size legend**
| Code | Story-point range | Calendar proxy |
|------|-------------------|----------------|
| S | < 5 pts | up to 1 week |
| M | 5–10 pts | 1–2 weeks |
| L | 10–20 pts | 2–4 weeks |
| XL | 20+ pts | 4+ weeks |

_Time is per-engineer, 100 % focused. RFD, QA, post-QA and UI/UX work are excluded._
EOF
}

deps_section() {
  local deps="$1"
  if [[ -n "$deps" ]]; then
    echo ""
    echo "**Dependencies (CSV index):** $deps"
  fi
}

optional_note() {
  local note="$1"
  if [[ -n "$note" ]]; then
    echo ""
    echo "> ⚠️ **Optional / lower priority:** $note"
  fi
}

# ===========================================================================
# Issue 1 – Public Plan Library
# ===========================================================================
create_issue \
  "[RFD-0038] 1. Public Plan Library" \
  "$(preamble)
## Description

Introduce a **public plan library** — a well-defined, versioned set of plan primitives that controllers and external consumers can use to express cluster-level and machine-level day-2-operation intent.

This library underpins virtually every other work item in this epic (items 2, 3, 6, 7, 11, 14 all depend on it directly).  Getting the API surface right here is the most important step for long-term maintainability.

### Acceptance criteria
- [ ] Package (or CRD group) for plan types is created and versioned under an appropriate API group (e.g. \`plan.cattle.io/v1\`)
- [ ] Existing plan types used by CAPR are migrated / aliased to the new library without breaking existing functionality
- [ ] Go client and generated DeepCopy / List types are produced
- [ ] Unit tests cover type helpers

**Dependencies:** none
**Size:** M (5–10 pts, ~1–2 weeks)
$(size_legend)"

# ===========================================================================
# Issue 2 – Plan Secret Schema Validation via Webhook
# ===========================================================================
create_issue \
  "[RFD-0038] 2. Plan Secret Schema Validation via Webhook" \
  "$(preamble)
## Description

Add **webhook-based validation** for plan secrets so that malformed or unsafe plan payloads are rejected before they reach the system-agent.

This prevents plan corruption bugs from being silently applied to downstream nodes and makes schema enforcement a hard gate.

### Acceptance criteria
- [ ] Validating webhook is registered for plan secrets (or the relevant plan CRD)
- [ ] Schema violations return a descriptive admission error
- [ ] Existing valid plans continue to pass without modification
- [ ] Unit + integration tests for the webhook handler

$(deps_section "1 (Public Plan Library)")
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 3 – Plan State Rework
# ===========================================================================
create_issue \
  "[RFD-0038] 3. Plan State Rework" \
  "$(preamble)
## Description

Refactor the internal representation of **plan state** so that:

- Applied, pending, and failed plan states are clearly distinguished
- State transitions are auditable
- The planner and system-agent converge on a single, consistent state model

This is a prerequisite for Snapshot Creation (#8), Certificate Rotation (#9), Encryption Key Rotation (#10), and Snapshot Restore (#12).

### Acceptance criteria
- [ ] Plan state enum / conditions are formally defined in the public plan library
- [ ] Planner writes state according to the new model
- [ ] System-agent reconciles against the new state fields
- [ ] Existing day-2-ops for provisioned v2 clusters continue to work

$(deps_section "1 (Public Plan Library)")
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 4 – Plan Cancellation
# ===========================================================================
create_issue \
  "[RFD-0038] 4. Plan Cancellation" \
  "$(preamble)
## Description

Allow an in-progress plan execution to be **cancelled** by an operator — useful when a day-2 op is taking too long, was triggered by mistake, or the cluster is in an unrecoverable partial state.

$(optional_note "Quality-of-life improvement; not strictly necessary for initial release.")

### Acceptance criteria
- [ ] A cancellation signal can be written to the plan or a derived resource
- [ ] The system-agent honours the signal and halts plan execution cleanly
- [ ] The planner transitions the plan state to \`Cancelled\`
- [ ] Partially applied plans emit a clear warning

$(deps_section "1 (Public Plan Library)")
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 5 – Day-2-op Data Preparation (feature, cluster context)
# ===========================================================================
create_issue \
  "[RFD-0038] 5. Day-2-op Data Preparation (feature, cluster context)" \
  "$(preamble)
## Description

Extract and normalise the **cluster-level data** needed to generate day-2 operation plans regardless of whether the cluster is provisioning-v2, CAPI/Turtles, or imported:

- Data directory (from \`rkeconfig.dataDirectories\`, \`rke2bootstrap\` annotations, or \`node.management.cattle.io\` status)
- CNI type (for rendering HTTP probes)
- Cluster driver / source of truth (\`clusters.cluster.x-k8s.io\` vs \`clusters.management.cattle.io\`)

### Acceptance criteria
- [ ] Helper functions that return data-dir and CNI for all three cluster types
- [ ] Fallback logic when values are absent
- [ ] Unit tests covering provisioning-v2, CAPI/Turtles, and imported cluster fixtures

**Dependencies:** none
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 6 – Beacon Implementation (CAPR, system-agent)
# ===========================================================================
create_issue \
  "[RFD-0038] 6. Beacon Implementation (CAPR, system-agent)" \
  "$(preamble)
## Description

Implement the \`Beacon\` CRD (\`beacons.plan.cattle.io\`) and its handling in **CAPR** and the **system-agent**.

A Beacon is a namespaced sentinel resource (one per cluster) that tells the system-agent where to find its machine-plan secret.  It is the bootstrapping handshake between Rancher and a node.

Key points from the RFD:
- Lives in the cluster's management namespace (e.g. \`c-m-xxxxxxx\`)
- Contains the minimum information required for plan lookup
- Created automatically when day-2-ops are enabled for a cluster
- CAPR reconciles it; system-agent watches it

### Acceptance criteria
- [ ] \`Beacon\` type defined in the public plan library (depends on #1)
- [ ] CAPR controller creates / updates the Beacon for provisioning-v2 and imported RKE2/K3s clusters
- [ ] system-agent is updated to discover its plan via the Beacon
- [ ] Beacon deletion triggers graceful cleanup of agent-side state
- [ ] Integration tests verify plan delivery end-to-end

$(deps_section "1 (Public Plan Library)")
**Size:** M (5–10 pts, ~1–2 weeks)
$(size_legend)"

# ===========================================================================
# Issue 7 – Beacon Implementation (CAPRKE2)
# ===========================================================================
create_issue \
  "[RFD-0038] 7. Beacon Implementation (CAPRKE2)" \
  "$(preamble)
## Description

Extend the Beacon implementation to **CAPRKE2** (Cluster API Provider RKE2).

CAPRKE2 uses per-node bootstrap resources (\`rke2bootstrap.cluster.x-k8s.io\`), so the Beacon wiring differs slightly from the CAPR path in issue #6.

### Acceptance criteria
- [ ] CAPRKE2 controller creates / reconciles Beacons for its clusters
- [ ] Per-node data directory from \`rke2bootstrap\` is surfaced correctly
- [ ] No regression in existing CAPRKE2 provisioning flows

$(deps_section "1 (Public Plan Library)")
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 8 – Snapshot Creation
# ===========================================================================
create_issue \
  "[RFD-0038] 8. Snapshot Creation" \
  "$(preamble)
## Description

Enable **etcd snapshot creation** for imported RKE2/K3s clusters using the new day-2 ops infrastructure.

Today this operation is only available for provisioning-v2 clusters.  This work item wires snapshot creation through the public plan library, Beacon-based plan delivery (items #6 / #3), and the data-preparation helpers (#5).

### Acceptance criteria
- [ ] Snapshot creation can be triggered on an imported cluster via the Rancher API
- [ ] A plan is generated, delivered via the Beacon, and executed by the system-agent
- [ ] Snapshot metadata is stored in \`etcdsnapshots.rke.cattle.io\`
- [ ] Existing provisioning-v2 snapshot creation is unaffected
- [ ] v2prov integration test for imported cluster snapshot creation

$(deps_section "3 (Plan State Rework), 5 (Day-2-op Data Preparation), 6 (Beacon – CAPR/system-agent)")
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 9 – Certificate Rotation
# ===========================================================================
create_issue \
  "[RFD-0038] 9. Certificate Rotation" \
  "$(preamble)
## Description

Enable **certificate rotation** for imported RKE2/K3s clusters.

The existing implementation (CAPR) removes \`/agent/pod-manifests\` to force certificate regeneration.  This work adapts that plan generation to imported clusters using the public plan library, Beacon-based delivery, and cluster-aware data-directory resolution.

### Acceptance criteria
- [ ] Certificate rotation can be triggered on an imported cluster
- [ ] Plan correctly references the data directory resolved from \`node.management.cattle.io\` status annotations
- [ ] HTTP probes (etcd, kube-apiserver, kube-scheduler, kube-controller-manager, kubelet, calico) are rendered and tracked
- [ ] Operation completes successfully on a real imported RKE2 cluster (integration test)
- [ ] Provisioning-v2 certificate rotation is not regressed

$(deps_section "3 (Plan State Rework), 5 (Day-2-op Data Preparation), 6 (Beacon – CAPR/system-agent)")
**Size:** M (5–10 pts, ~1–2 weeks)
$(size_legend)"

# ===========================================================================
# Issue 10 – Encryption Key Rotation
# ===========================================================================
create_issue \
  "[RFD-0038] 10. Encryption Key Rotation" \
  "$(preamble)
## Description

Enable **encryption key rotation** for imported RKE2/K3s clusters.

Similar scope to Certificate Rotation (#9) but with its own plan steps (running encryption-key-rotation scripts, etc.).

### Acceptance criteria
- [ ] Encryption key rotation can be triggered on an imported cluster
- [ ] Plan steps match the existing CAPR implementation adapted for imported cluster data sources
- [ ] Integration test verifies end-to-end key rotation on an imported RKE2 cluster
- [ ] Provisioning-v2 encryption key rotation is not regressed

$(deps_section "3 (Plan State Rework), 5 (Day-2-op Data Preparation), 6 (Beacon – CAPR/system-agent)")
**Size:** M (5–10 pts, ~1–2 weeks)
$(size_legend)"

# ===========================================================================
# Issue 11 – In-place Updates (CAPRKE2)
# ===========================================================================
create_issue \
  "[RFD-0038] 11. In-place Updates (CAPRKE2)" \
  "$(preamble)
## Description

Implement **in-place Kubernetes version upgrades** for CAPRKE2-managed clusters — as opposed to rolling node replacement.

This is the most complex work item in the epic.  It requires:
- The public plan library (#1) for plan expression
- Webhook validation (#2) to prevent unsafe upgrade plans
- In-place update contracts (#20) defining the CAPRKE2 API surface
- Plan State Rework (#3) for tracking upgrade progress
- Data preparation (#5) for version/image resolution
- Beacon implementations (#7 for CAPRKE2, #6 for system-agent)
- The locking mechanism (#18) to prevent concurrent operations

### Acceptance criteria
- [ ] CAPRKE2 controller can initiate an in-place upgrade plan
- [ ] system-agent executes the upgrade safely (drains workloads, upgrades binaries, validates probes)
- [ ] Upgrade can be paused, resumed, and cancelled
- [ ] Failure leaves the cluster in a recoverable state
- [ ] Integration tests cover single-node and multi-node upgrade scenarios

$(deps_section "1 (Public Plan Library), 2 (Webhook Validation), 3 (Plan State Rework), 5 (Data Preparation), 7 (Beacon – CAPRKE2), 18 (Locking), 20 (In-place Update Contracts)")
**Size:** L (10–20 pts, ~2–4 weeks)
$(size_legend)"

# ===========================================================================
# Issue 12 – Snapshot Restore
# ===========================================================================
create_issue \
  "[RFD-0038] 12. Snapshot Restore" \
  "$(preamble)
## Description

Enable **etcd snapshot restore** for imported RKE2/K3s clusters.

This is the most destructive day-2 operation and therefore the most complex to implement safely.  The existing CAPR restore plan:
1. Deletes \`/server/db/etcd\`
2. Creates a tombstone file
3. Deletes \`/server/tls\` and \`/server/cred\`
4. Runs \`RKE2_DATA_DIR=%s rke2-killall.sh\`
5. Runs the etcd-restore script

Restore depends on Snapshot Creation (#8) being functional first.

### Acceptance criteria
- [ ] Snapshot restore can be triggered on an imported cluster referencing a previously created snapshot
- [ ] All restore steps execute in order with proper guardrails
- [ ] Cluster health probes gate completion
- [ ] Integration test verifies restore on an imported RKE2 cluster
- [ ] Provisioning-v2 snapshot restore is not regressed

$(deps_section "3 (Plan State Rework), 5 (Day-2-op Data Preparation), 6 (Beacon – CAPR/system-agent), 8 (Snapshot Creation)")
**Size:** L (10–20 pts, ~2–4 weeks)
$(size_legend)"

# ===========================================================================
# Issue 13 – Lifecycle Hooks
# ===========================================================================
create_issue \
  "[RFD-0038] 13. Lifecycle Hooks" \
  "$(preamble)
## Description

Add **lifecycle hooks** that fire before and after each day-2 operation (snapshot create, cert rotation, etc.).  Primarily useful for debugging, investigation, and potentially for custom pre/post scripts.

$(optional_note "Not strictly required for the initial release; useful for investigation and extensibility.")

### Acceptance criteria
- [ ] Pre- and post-operation hooks can be defined on the plan or operation resource
- [ ] Hook failures can be configured to block or warn-only
- [ ] Hooks are visible in the plan status / events

$(deps_section "8 (Snapshot Creation)")
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 14 – Plan Pausing
# ===========================================================================
create_issue \
  "[RFD-0038] 14. Plan Pausing" \
  "$(preamble)
## Description

Allow an operator to **pause** plan execution — halting further application of instructions without cancelling the operation — and then resume it.

$(optional_note "Quality-of-life improvement; not strictly necessary for initial release.")

### Acceptance criteria
- [ ] A pause field / annotation can be set on the plan resource
- [ ] The planner and system-agent honour the pause and do not advance to the next instruction
- [ ] Resuming the plan continues from where it stopped (idempotent)
- [ ] Status clearly reflects the \`Paused\` state

$(deps_section "1 (Public Plan Library), 3 (Plan State Rework)")
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 15 – Operation Pausing
# ===========================================================================
create_issue \
  "[RFD-0038] 15. Operation Pausing" \
  "$(preamble)
## Description

Extend the pausing concept from plan-level (#14) to **operation-level** — pausing an entire day-2 operation (e.g. snapshot restore) across all its constituent plans.

$(optional_note "Quality-of-life improvement; not strictly necessary for initial release.")

### Acceptance criteria
- [ ] An operation can be paused and resumed atomically
- [ ] Pausing an operation pauses all its constituent machine-level plans
- [ ] The operation status reflects \`Paused\`

$(deps_section "8 (Snapshot Creation), 14 (Plan Pausing)")
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 16 – Operation Cancellation
# ===========================================================================
create_issue \
  "[RFD-0038] 16. Operation Cancellation" \
  "$(preamble)
## Description

Allow an operator to **cancel** an entire in-progress day-2 operation (not just a single plan).

$(optional_note "Quality-of-life improvement; not strictly necessary for initial release.")

### Acceptance criteria
- [ ] An operation-level cancellation signal propagates to all machine-level plans
- [ ] All constituent plans transition to \`Cancelled\` (via plan cancellation from #4)
- [ ] The operation resource records the cancellation reason and timestamp

$(deps_section "8 (Snapshot Creation), 4 (Plan Cancellation)")
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 17 – Data Extraction and Probes
# ===========================================================================
create_issue \
  "[RFD-0038] 17. Data Extraction and Probes" \
  "$(preamble)
## Description

Implement **data extraction** from completed plans (stdout/stderr of periodic instructions) and enrich the probe framework so that all key cluster components are monitored during and after day-2 operations.

Probes that must be rendered for RKE2/K3s clusters:
- \`etcd\`
- \`kube-apiserver\`
- \`kube-scheduler\`
- \`kube-controller-manager\`
- \`kubelet\`
- \`calico\` (and other common CNIs)

Probe success must be tracked independently of plan-apply success (a plan can succeed while probes are still unhealthy).

### Acceptance criteria
- [ ] Periodic instruction outputs are surfaced in the plan/operation status
- [ ] Probes are rendered correctly for imported clusters (CNI determined via data-prep helpers)
- [ ] Probe status is polled and reported by the system-agent
- [ ] Dashboards / status fields distinguish \`PlanApplied\` from \`ProbesHealthy\`

$(deps_section "8 (Snapshot Creation)")
**Size:** M (5–10 pts, ~1–2 weeks)
$(size_legend)"

# ===========================================================================
# Issue 18 – Locking Mechanism
# ===========================================================================
create_issue \
  "[RFD-0038] 18. Locking Mechanism" \
  "$(preamble)
## Description

Introduce a **per-cluster operation lock** to prevent concurrent day-2 operations from racing and corrupting cluster state.

The lock must be:
- Checked by the webhook (reject enabling/disabling day-2-ops while an operation is in progress)
- Acquired before any operation plan is written
- Released on operation completion, cancellation, or error

### Acceptance criteria
- [ ] Lock resource or field is defined in the public plan library or on the management cluster object
- [ ] Webhook enforces the lock for annotation changes (per the RFD webhook requirements)
- [ ] Planner refuses to start a new operation if the lock is held
- [ ] Lock is always released, even on failure (no stuck locks)

$(deps_section "2 (Webhook Validation), 6 (Beacon – CAPR/system-agent), 7 (Beacon – CAPRKE2)")
**Size:** S (< 5 pts, up to 1 week)
$(size_legend)"

# ===========================================================================
# Issue 19 – Scaling Improvements
# ===========================================================================
create_issue \
  "[RFD-0038] 19. Scaling Improvements" \
  "$(preamble)
## Description

Improve the performance of the day-2 ops infrastructure for **large clusters** (many nodes, many concurrent operations across multiple clusters).

$(optional_note "Performance improvement; not strictly necessary for the initial release.")

Areas of focus:
- Reduce planner reconcile churn for unchanged plans
- Improve plan-secret watch efficiency in the system-agent
- Ensure the Beacon controller scales to thousands of clusters

### Acceptance criteria
- [ ] Benchmarks establish a baseline for planner throughput
- [ ] Identified bottlenecks are resolved
- [ ] No visible latency increase in existing day-2-ops at scale

$(deps_section "6 (Beacon – CAPR/system-agent)")
**Size:** M (5–10 pts, ~1–2 weeks)
$(size_legend)"

# ===========================================================================
# Issue 20 – In-place Update Contracts (CAPRKE2)
# ===========================================================================
create_issue \
  "[RFD-0038] 20. In-place Update Contracts (CAPRKE2)" \
  "$(preamble)
## Description

Define and implement the **API contracts** that govern in-place Kubernetes version upgrades in CAPRKE2 — the public-facing spec changes, status conditions, and compatibility guarantees that downstream consumers (UI, CLI, other controllers) can rely on.

This is foundational work for the in-place upgrades implementation (#11) and should be designed with future extensibility in mind.

### Acceptance criteria
- [ ] API types for in-place upgrade spec and status are defined and versioned
- [ ] Upgrade compatibility rules are documented (minimum supported version delta, etc.)
- [ ] Existing CAPRKE2 API is not broken
- [ ] CRD validation rules enforce the contracts

**Dependencies:** none
**Size:** L (10–20 pts, ~2–4 weeks)
$(size_legend)"

echo ""
echo "✅  Done. All 20 RFD-0038 issues submitted to $REPO."
