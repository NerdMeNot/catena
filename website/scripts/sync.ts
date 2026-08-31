// Derives the site's content from the repository, so the docs on the web
// and the docs in the tree cannot disagree.
//
// What it reads:
//   version.go          the released version — the SINGLE source of truth
//   docs/0N-*.md        the prose docs, in their numbered order
//   examples/*/main.go  the runnable examples, embedded verbatim
//   CHANGELOG.md        the release notes page
//
// What it writes: src/content/docs/**.md (Starlight pages with frontmatter)
// and src/data/site.json (version and counts the layout reads).
//
// The output is COMMITTED, and CI fails when it is stale — so a deploy
// builds exactly the files that were reviewed, and this script never runs
// on the deploy path. `bun run sync` after changing anything it reads.

import { readdir, readFile, writeFile, mkdir, rm } from 'node:fs/promises'
import { join } from 'node:path'

const repo = join(import.meta.dir, '..', '..')
const outDir = join(import.meta.dir, '..', 'src', 'content', 'docs')
const dataDir = join(import.meta.dir, '..', 'src', 'data')

/**
 * The version this site documents: the latest RELEASED one, which is the
 * latest version heading in the changelog — not `version.go`, which
 * between releases names the *next* version with a -dev suffix.
 *
 * The two are cross-checked: on a release commit (no -dev suffix) they
 * must agree, which is what stops a release shipping a site that
 * describes a different version than the code in it.
 */
async function documentedVersion(): Promise<string> {
  const changelog = await readFile(join(repo, 'CHANGELOG.md'), 'utf8')
  const released = changelog.match(/^## \[(\d+\.\d+\.\d+)\]/m)
  if (!released) throw new Error('CHANGELOG.md: no released version heading found')

  const src = await readFile(join(repo, 'version.go'), 'utf8')
  const constant = src.match(/^const Version = "(.+)"$/m)
  if (!constant) throw new Error('version.go: no Version constant found')

  const isDev = constant[1].endsWith('-dev')
  if (!isDev && constant[1] !== released[1]) {
    throw new Error(
      `version.go says ${constant[1]} but the newest CHANGELOG entry is ${released[1]} — ` +
        'a release must not ship docs describing a different version.',
    )
  }
  return released[1]
}

/** Title and one-line summary from a doc's first heading and paragraph. */
function titleOf(md: string, fallback: string): { title: string; body: string } {
  const lines = md.split('\n')
  if (lines[0]?.startsWith('# ')) {
    return { title: lines[0].slice(2).trim(), body: lines.slice(1).join('\n').trim() }
  }
  return { title: fallback, body: md }
}

/** First paragraph, flattened — Starlight uses it as the page description. */
function summarize(body: string): string {
  const para = body
    .split('\n\n')
    .find((p) => p.trim() && !p.startsWith('#') && !p.startsWith('|') && !p.startsWith('```'))
  if (!para) return ''
  return para.replace(/\s+/g, ' ').replace(/\[([^\]]+)\]\([^)]+\)/g, '$1').replace(/[`*]/g, '').trim()
}

/**
 * Rewrites in-repo markdown links to their site routes. Docs link to each
 * other by filename (`02-concepts.md`) and to code by repo path; on the
 * site the former become routes and the latter become GitHub links.
 */
function rewriteLinks(md: string, repoUrl: string): string {
  return md
    // sibling docs: 02-concepts.md -> /guides/concepts
    .replace(/\]\((?:\.\.\/docs\/)?(\d\d)-([a-z-]+)\.md(#[^)]*)?\)/g, (_, _n, slug, hash) =>
      `](/guides/${slug}/${hash ?? ''})`)
    // examples: ../examples/01-basics/main.go -> /examples/basics
    .replace(/\]\((?:\.\.\/)?examples\/\d\d-([a-z-]+)(?:\/main\.go)?\)/g, (_, slug) =>
      `](/examples/${slug}/)`)
    // repo files that have no site page: link to GitHub
    .replace(/\]\((?:\.\.\/)?((?:CHANGELOG|LICENSE|README)\.md|[a-z_]+\.go|[a-z_]+_test\.go)\)/g,
      (_, file) => `](${repoUrl}/blob/main/${file})`)
    .replace(/\]\(docs\/README\.md\)/g, '](/guides/getting-started/)')
    .replace(/\]\(examples\/\)/g, '](/examples/)')
}

function frontmatter(fields: Record<string, string | number | undefined>): string {
  const yaml = Object.entries(fields)
    .filter(([, v]) => v !== undefined && v !== '')
    .map(([k, v]) => (typeof v === 'number' ? `${k}: ${v}` : `${k}: ${JSON.stringify(v)}`))
    .join('\n')
  return `---\n${yaml}\n---\n\n`
}

const repoUrl = 'https://github.com/NerdMeNot/catena'

// Where the site is actually served from. The canonical URL and the
// sitemap are built from this, so it has to name a host that resolves —
// pointing them at a domain that is not connected yet tells search
// engines the real pages live somewhere unreachable. Change it here, in
// one place, if the site moves to a custom domain.
const siteUrl = 'https://catena-docs.pages.dev'

// The release that froze the v1 API. Pinned rather than derived from the
// documented version: later v1 releases inherit that promise, they do not
// re-make it. v1.0.0 was published briefly and withdrawn, so the first
// release anyone can depend on is this one.
const firstStable = '1.1.0'

async function main() {
  const version = await documentedVersion()

  // Only the directories this script owns are cleared. Archived versions
  // (src/content/docs/1-1/ and friends) are snapshots of a past release
  // that no longer exists in this working tree, so nothing here can
  // regenerate them — wiping the whole content directory would destroy
  // them permanently on the next sync.
  const generated = ['guides', 'operators', 'examples', 'reference']
  for (const dir of generated) {
    await rm(join(outDir, dir), { recursive: true, force: true })
    await mkdir(join(outDir, dir), { recursive: true })
  }
  await mkdir(dataDir, { recursive: true })

  // ---- guides, from docs/0N-*.md ----
  const docFiles = (await readdir(join(repo, 'docs')))
    .filter((f) => /^\d\d-.*\.md$/.test(f))
    .sort()

  for (const file of docFiles) {
    const [, order, slug] = file.match(/^(\d\d)-(.+)\.md$/)!
    const raw = await readFile(join(repo, 'docs', file), 'utf8')
    const { title, body } = titleOf(raw, slug)
    const page = frontmatter({
      title,
      description: summarize(body),
      sidebar: undefined,
    }).replace('---\n\n', `sidebar:\n  order: ${Number(order)}\n---\n\n`)
    await writeFile(join(outDir, 'guides', `${slug}.md`), page + rewriteLinks(body, repoUrl) + '\n')
  }

  // ---- operator reference, generated by internal/gen/opdocs ----
  // Those pages are themselves generated from the library's doc comments
  // and its verified Example functions, so this step only reshapes them
  // for the site; it never authors anything.
  const opFiles = (await readdir(join(repo, 'docs', 'operators')))
    .filter((f) => /^\d\d-.*\.md$/.test(f))
    .sort()

  for (const file of opFiles) {
    const [, order, slug] = file.match(/^(\d\d)-(.+)\.md$/)!
    const raw = await readFile(join(repo, 'docs', 'operators', file), 'utf8')
    const { title, body } = titleOf(raw, slug)
    const page = frontmatter({
      title,
      description: summarize(body),
    }).replace('---\n\n', `sidebar:\n  order: ${Number(order)}\n---\n\n`)
    await writeFile(join(outDir, 'operators', `${slug}.md`), page + rewriteLinks(body, repoUrl) + '\n')
  }

  // ---- examples, each embedding its program verbatim ----
  const exampleDirs = (await readdir(join(repo, 'examples'), { withFileTypes: true }))
    .filter((e) => e.isDirectory())
    .map((e) => e.name)
    .sort()

  const exampleIndex = await readFile(join(repo, 'examples', 'README.md'), 'utf8')
  const blurbs = new Map<string, string>()
  for (const m of exampleIndex.matchAll(/\|\s*\[([a-z-]+)\]\([^)]+\)\s*\|\s*(.+?)\s*\|/g)) {
    blurbs.set(m[1], m[2])
  }

  for (const dir of exampleDirs) {
    const [, order, slug] = dir.match(/^(\d\d)-(.+)$/)!
    const code = await readFile(join(repo, 'examples', dir, 'main.go'), 'utf8')
    // The file's own header comment is the example's explanation; it is
    // written for a reader, so the page uses it rather than restating it.
    const header = code
      .split('\n')
      .slice(0, code.split('\n').findIndex((l) => l.startsWith('package ')))
      .filter((l) => l.startsWith('//'))
      .map((l) => l.replace(/^\/\/ ?/, ''))
      .join(' ')
      .trim()
    const page =
      frontmatter({
        title: slug.replace(/(^|-)(\w)/g, (_, s, c) => (s ? ' ' : '') + c.toUpperCase()),
        description: blurbs.get(slug) ?? header.slice(0, 160),
      }).replace('---\n\n', `sidebar:\n  order: ${Number(order)}\n---\n\n`) +
      `${header}\n\nRun it: \`go run ./examples/${dir}\`\n\n` +
      '```go\n' + code.trimEnd() + '\n```\n\n' +
      `[View on GitHub](${repoUrl}/blob/main/examples/${dir}/main.go)\n`
    await writeFile(join(outDir, 'examples', `${slug}.md`), page)
  }

  const mod = 'github.com/NerdMeNot/catena'

  // ---- reference: versioning ----
  // Written here rather than kept as a static page so the version it
  // quotes is the version the rest of the site was built from.
  await writeFile(
    join(outDir, 'reference', 'versioning.md'),
    frontmatter({
      title: 'Versioning',
      description: `These docs describe catena v${version}. What the version guarantees, how to pin it, and how the docs track releases.`,
    }) +
      `## What these docs describe

**This site documents catena v${version}** — the version shown in the header,
and the one \`go get ${mod}\` resolves to. That number is read from
the library's own \`Version\` constant when the site is built, so the docs
and the code cannot disagree about which release is being described; CI
fails the build if they drift.

## What v1 guarantees

catena follows [semantic versioning](https://semver.org). \`v${firstStable}\`,
the first supported release, froze
the API: within v1, operators keep their names, signatures, and documented
behaviour — including the contracts that are easy to depend on without
noticing, such as which operators buffer, which terminate early, and what
each does with errors. Those are specified per operator and enforced by the
conformance suite, so they are part of the compatibility promise rather
than incidental behaviour.

Anything that would break a working program waits for v2.

## Pinning

\`\`\`sh
go get ${mod}@v${version}
\`\`\`

Go modules pin by default — your \`go.mod\` records the exact version and
\`go.sum\` its checksum, so a build is reproducible without further effort.

### Retracted versions

The \`v0.x\` development checkpoints, and \`v1.0.0\` — published briefly and
withdrawn — are **retracted** in \`go.mod\`. Module proxies cache versions
permanently, so those tags may still appear in \`go list -m -versions\`
output; the retraction marks them as withdrawn, and \`go get\` will not
select one. Use v${firstStable} or later.

## How the docs track releases

Today catena has one released version, so this site documents it directly
and needs no version switcher. When a second release exists, the previous
one is archived at \`/${version.split('.').slice(0, 2).join('-')}/…\` and a
version selector appears in the header
— the machinery ([starlight-versions](https://starlight-versions.vercel.app))
is already a dependency, and the procedure is written down in the
repository's releasing guide.

Until then, the rule that matters is the one above: the version in the
header is read from the source, not typed by hand.
`,
  )

  // ---- reference: changelog ----
  const changelog = await readFile(join(repo, 'CHANGELOG.md'), 'utf8')
  const { body: clBody } = titleOf(changelog, 'Changelog')
  await writeFile(
    join(outDir, 'reference', 'changelog.md'),
    frontmatter({
      title: 'Changelog',
      description: 'Every released version of catena and what changed in it.',
    }) + rewriteLinks(clBody, repoUrl) + '\n',
  )

  // ---- data the layout reads ----
  await writeFile(
    join(dataDir, 'site.json'),
    JSON.stringify(
      {
        version,
        repoUrl,
        siteUrl,
        module: 'github.com/NerdMeNot/catena',
        guides: docFiles.length,
        operators: opFiles.length,
        examples: exampleDirs.length,
      },
      null,
      2,
    ) + '\n',
  )

  console.log(
    `sync: v${version} — ${docFiles.length} guides, ${opFiles.length} operator pages, ` +
      `${exampleDirs.length} recipes, 2 reference pages`,
  )
}

await main()
