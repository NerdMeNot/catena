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
   git tag -a v1.1.0 -m "catena v1.1.0"
   git push origin v1.1.0
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
`catena-docs`, live at <https://catena-docs.pages.dev>.

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

The site documents one release at a time and shows which in its header.
When a **second** release exists, archive the previous one so readers on
an older version still have its docs:

1. Add the plugin to `website/astro.config.mjs` (it is already a
   dependency):

   ```js
   import starlightVersions from 'starlight-versions'

   plugins: [starlightVersions({ versions: [{ slug: '1-1' }] })]
   ```

2. Run `bun run astro dev` once. The plugin snapshots the current pages
   into `src/content/docs/1-1/` — commit that directory.
3. From then on, `src/content/docs/**` is the *next* version and the
   snapshot is v1.1; a version selector appears in the header
   automatically.

Do this at the release that creates the second version, not before: the
plugin requires at least one archived version, and archiving the only
release you have would mean the site had no current one.

## Versioning policy

Semantic versioning. `v1.1.0` is the first supported release and freezes
the API; anything removed or changed incompatibly waits for a major
version that nobody is in a hurry to have. Earlier tags were withdrawn and
are retracted in `go.mod`.
