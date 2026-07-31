# Vendored: buildkit-cache-dance @ v2.1.4

Source: https://github.com/reproducible-containers/buildkit-cache-dance
Tag: v2.1.4 (commit 4b2444fec0c0fb9dbf175a96c094720a692ef810)
License: Apache-2.0 (see LICENSE in this directory)

Vendored (not referenced directly) because this action isn't on
rancher/rancher's GitHub Actions allow-list.

## Why this version, alongside the v3.4.0 vendor in the sibling
`buildkit-cache-dance/` directory

`reproducible-containers/buildkit-cache-dance#33` ("Stored cache is empty",
open since Jun 2024, unresolved) has multiple independent reports of
downgrading to v2.1.4 as a workaround -- e.g.
https://github.com/youtalk/autoware/pull/58/files. v2.1.4 predates the
current `cache-map` (multi-path, per-mount `id`) architecture: it only
supports a single `cache-source`/`cache-target` pair and its generated
Dockerfile `RUN --mount=type=cache,target=...` has **no explicit `id=`** --
BuildKit defaults the cache-mount id to the `target` path in that case. This
is a different code path from v3.x, not just a version bump, so it's worth
testing independently of our v3.4.0-based findings (see
`cache-benchmark-cold-cachemount-dance-v2` job in `pull-request.yml`, and
`rancher-go-builder-mount-dance-v2`/`server-build-mount-dance-v2` stages in
`package/Dockerfile`, which deliberately use a bare `--mount=type=cache,
target=/root/.cache` with no `id=` to match this action's implicit-id
behavior).

Reports of v2.1.4 fixing the issue were NOT unanimous in that GitHub issue
thread -- at least one reporter (`psypuff`) said downgrading to v2.1.4 did
NOT help for them, and traced the real cause (in their case) to combining
cache mounts with `cache-from`/`cache-to` layer caching. Our benchmark jobs
don't use `cache-from`/`cache-to` at all, so that specific interaction
shouldn't apply here -- but this is still a real, untested-by-us hypothesis,
not a confirmed fix, hence testing it empirically rather than assuming it
works.
