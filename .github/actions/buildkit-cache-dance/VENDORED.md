# Vendored: buildkit-cache-dance

This is a vendored copy of
[reproducible-containers/buildkit-cache-dance](https://github.com/reproducible-containers/buildkit-cache-dance)
at tag `v3.4.0` (commit-pinned via the release tarball, see below).

## Why vendored instead of referenced directly

`rancher/rancher`'s GitHub Actions policy only allows actions from an
explicit allow-list of repos/orgs (enterprise-owned, GitHub-created, or a
short list of trusted orgs). `reproducible-containers/*` is not on that
list, and workflows referencing `reproducible-containers/buildkit-cache-dance@v3`
directly fail with:

```
Error
The action reproducible-containers/buildkit-cache-dance@v3 is not allowed
in rancher/rancher because all actions must be from a repository owned by
your enterprise, created by GitHub, or match one of the patterns: ...
```

Vendoring the action's compiled output under `.github/actions/` and
referencing it as `./.github/actions/buildkit-cache-dance` sidesteps this,
since local composite actions (referenced via a relative path in the same
repo) aren't subject to the external-action allow-list.

## What's here

- `action.yml` — unmodified action manifest from the upstream release.
- `dist/index.js` (+ `.map`) — the upstream project's pre-built Node
  bundle (it ships compiled output in its `dist/` dir at every tagged
  release; there is no separate build step required to vendor it).
- `LICENSE` — upstream's Apache-2.0 license, kept verbatim per its terms.

## Updating this vendored copy

1. Check the latest release tag: https://github.com/reproducible-containers/buildkit-cache-dance/releases
2. Download and extract the release tarball:
   ```bash
   curl -sL https://github.com/reproducible-containers/buildkit-cache-dance/archive/refs/tags/vX.Y.Z.tar.gz | tar xz
   ```
3. Replace `action.yml`, `dist/`, and `LICENSE` in this directory with the
   new release's copies.
4. Update the version note at the top of this file.
5. Diff `dist/index.js` against the previous version's before committing,
   as a sanity check that nothing unexpected changed (it's a compiled
   bundle, so a real code review of upstream's `src/` on GitHub is more
   informative than diffing the minified output).
