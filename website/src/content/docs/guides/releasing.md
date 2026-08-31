---
title: "Releasing"
description: "Go modules resolve from git tags, so the tag is the release — the workflow's job is to refuse to publish one that shouldn't exist, then create the GitHub release for the one that should."
sidebar:
  order: 6
---

Go modules resolve from git tags, so the tag *is* the release — the
workflow's job is to refuse to publish one that shouldn't exist, then
create the GitHub release for the one that should.

## Cutting a release

Between releases, `Version` carries a `-dev` suffix; a release drops it.

1. Set `Version` in `version.go` to the release version and give the
   release its `CHANGELOG.md` section, with today's date and a link at
   the bottom.
2. Commit, push, and let CI go green on `main`.
3. Tag and push:

   ```sh
   git tag -a v1.2.0 -m "catena v1.2.0"
   git push origin v1.2.0
   ```

4. After the release publishes, bump `Version` back to the next
   `-dev` version.

**Tag only when you mean it.** A pushed tag on a public Go module is
cached by the module proxy on first fetch, permanently — the checksum
database is append-only. Deleting a tag does not delete the cached
version; a tag that should not exist can only be retracted in `go.mod`,
never truly withdrawn. So a tag is cut once, at the end, on purpose.

The `Release` workflow then:

- **Verifies** the tag agrees with `Version` — a release whose code
  reports a different version than the tag that produced it is worse than
  no release, so nothing is published until they match.
- **Runs the full gate** on the tagged commit: gofmt, vet,
  generate-idempotency (`list_gen.go` must be current), the race-enabled
  suite, and the 100% module-wide coverage check.
- **Publishes** the GitHub release, with the tag's `CHANGELOG.md` section
  as its notes.

No artifacts are built: this is a library, and `go get` fetches the tag
directly. A failed publish can be retried without moving the tag via the
workflow's manual dispatch (`Actions → Release → Run workflow`, giving the
existing tag).

## The website

`website/` builds a documentation site from this repository:
`scripts/sync.ts` turns `docs/`, `examples/` and `CHANGELOG.md` into its
pages, and the generated output is committed. CI fails if it is stale, so
a deploy always publishes what was reviewed rather than something
generated unseen.

The `Docs` workflow publishes it to the Cloudflare Pages project
`catena-docs`, live at <https://catena.nerdmenot.in> (the project also
answers on its `catena-docs.pages.dev` subdomain, but the custom domain is
the canonical one — see `siteUrl` in `website/scripts/sync.ts`).

**Only a released tag is ever deployed.** The site names the version it
documents, and its content is derived from the tree it was built from, so
publishing `main` would put the two in disagreement as soon as anything
lands unreleased — the site would describe operators that `go get` cannot
give the reader. A push to `main` therefore builds the site and runs every
check against it and stops; the deploy happens when a release is
published, and builds that tag. To republish the current release by hand —
after a failed deploy, or if the site drifts — run `Actions → Docs → Run
workflow`, leaving the tag blank to mean "the latest release".

Deploying needs `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` as
repository secrets; without them the workflow still builds and checks the
site, then skips the deploy with a notice rather than failing — so a
fork's pull request is not red for want of credentials it cannot have. Run
`cd website && bun run dev` to read the site locally, which does show the
working tree.

After changing anything the site derives from, run:

```sh
cd website && bun run sync
```

and commit the result. Publishing a release republishes the site
automatically — the version it displays comes from the newest changelog
entry, and the deploy refuses to publish if that disagrees with what the
generated content says.

### Versioning the website

The site serves the current release at the root and each archived series
under its own prefix, with a switcher in the header. The `v1.2.x` series is
archived at `/1-2/` and `v1.1.x` at `/1-1/`; their pages carry a notice
saying so and linking to the current one.

**One archive per minor series, not per tag.** Three reasons:

- pkg.go.dev already archives every tag's API — signatures and doc
  comments at any exact version, for free. What this site adds is the
  prose, and that does not change between patches.
- Under semantic versioning a patch cannot change the API, so a patch's
  operator reference is its minor's. Archiving patches would publish
  near-duplicates.
- A published URL is permanent. Once `/1-2/` exists and someone links it,
  removing it breaks their link — so archive at the coarsest granularity
  that is still useful, because adding is cheap and removing is not.

Which gives the rule:

- Archive a minor series when the **next minor** ships, snapshotting the
  last tag in the outgoing series. Label it for the series (`v1.2.x`), not
  for that tag.
- A patch release archives nothing: it *is* the current release, so it
  refreshes the current docs.
- **Never archive a retracted version.** `v1.0.0` and `v0.1.0`–`v0.3.0`
  are retracted in `go.mod`; a switcher entry for a version `go get`
  refuses to select is worse than no entry.
- Past about six entries the control stops being usable. Drop the oldest
  from the switcher then, but leave their pages served so existing links
  keep working.

Do not skip a series because it changed little. `v1.2.0` was left out on
that reasoning and the switcher jumped from `v1.3.0` to `v1.1.x`, which
reads as a fault — the reader cannot see the reasoning, only the gap. It
also turned out to be wrong: that series added `BottomN`.

To archive a series when you cut the next minor:

1. **Take the snapshot from the tag, not from the working tree.** The
   plugin will happily snapshot whatever is currently in
   `src/content/docs/`, but that is the *new* release — filing it under
   the old version's name would publish the new API as though it were the
   old one. Copy the real thing out of the tag instead:

   ```sh
   dest=website/src/content/docs/1-2
   mkdir -p "$dest"
   git archive v1.2.0 website/src/content/docs | tar -x --strip-components=4 -C "$dest"
   ```

2. **Record that version's sidebar** in
   `website/src/content/versions/1-2.json`, as
   `{"sidebar": [...], "excluded": []}`, copying the `sidebar` from that
   tag's `astro.config.mjs`. The plugin refuses to build without it.
3. **List it** in `astro.config.mjs`, newest first:

   ```js
   versions: [{ slug: '1-2', label: 'v1.2.x' }, { slug: '1-1', label: 'v1.1.x' }]
   ```

4. Build once and check the archived page shows the outdated-version
   notice, the switcher offers both versions, and the archived operator
   reference does *not* mention anything added since.

Two things about the setup are worth knowing before you touch it. The
archived directories are snapshots of a tree that no longer exists, so
nothing can regenerate them — `scripts/sync.ts` therefore clears only the
four directories it owns rather than all of `src/content/docs/`. And a
component override in `astro.config.mjs` beats a plugin's, so `Header` and
`PageTitle` render the plugin's `VersionSelect`, `VersionSearch` and
`VersionNotice` themselves; drop those imports and the switcher and the
outdated-version notice disappear silently.

### Publishing a change to the site itself

Deploying is normally tied to releases, which leaves a gap: a change to
the site's own machinery — a new archived version, a layout fix — has
nothing to do with the library and should not wait for one.

`Actions → Docs → Run workflow` takes a ref, and it may be `main`. What
keeps that honest is a guard in the workflow: the deploy fails unless
`docs/operators/` at that ref is identical to the latest release tag. That
directory is generated from the library's doc comments and its verified
examples, so if it matches, the site describes exactly the API `go get`
delivers, whatever else has changed. If it does not match, the library has
moved and the answer is to cut a release rather than publish the branch.

## Versioning policy

Semantic versioning. `v1.2.0` is the current release. `v1.1.0` was the
first supported one and froze the API; anything removed or changed incompatibly waits for a major
version that nobody is in a hurry to have. Earlier tags were withdrawn and
are retracted in `go.mod`.
