// The catena documentation site.
//
// Starlight owns `src/content/docs/**`, which is generated from the
// repository by `scripts/sync.ts` — the guides, examples and changelog on
// this site are the ones in the tree, not a second copy that drifts. The
// landing page is a plain Astro route with its own layout, because a docs
// framework's default shell is the fastest way to make a project look like
// every other project.
//
// ## Versioning
//
// The current version comes from `version.go` through the sync script, so
// no version string is written down twice. Past releases are archived
// under `src/content/docs/<slug>/` and listed below; `starlight-versions`
// serves them and adds the switcher. Those directories are snapshots of a
// release that no longer exists in this tree — nothing regenerates them,
// which is why `scripts/sync.ts` clears only the directories it owns.
// The procedure for archiving one is in docs/06-releasing.md.
import { defineConfig } from 'astro/config'
import starlight from '@astrojs/starlight'
import starlightVersions from 'starlight-versions'

import site from './src/data/site.json' with { type: 'json' }

export default defineConfig({
  // Where the site is served from, so the canonical URL and the sitemap
  // name a host that resolves. Set in scripts/sync.ts.
  site: site.siteUrl,
  integrations: [
    starlight({
      title: 'catena',
      description:
        'Lazy, fully typed sequence pipelines for Go — Kotlin and LINQ ergonomics on ' +
        'iter.Seq, with every operator’s memory and termination behaviour specified and tested.',
      favicon: '/icon.svg',
      customCss: ['./src/styles/theme.css'],
      // Code reads as a terminal in both site themes. A light syntax theme
      // washes out on paper, and switching themes mid-scroll is worse than
      // committing to one.
      expressiveCode: {
        themes: ['github-dark'],
        styleOverrides: {
          borderRadius: '4px',
          borderColor: 'transparent',
          codeFontFamily: 'var(--catena-mono)',
        },
      },
      plugins: [
        starlightVersions({
          // Labelled with the release it documents rather than "Latest",
          // so the switcher names a version a reader can `go get`.
          current: { label: `v${site.version}` },
          // Archived releases, newest first. Each needs its snapshot
          // committed under src/content/docs/<slug>/.
          versions: [{ slug: '1-1', label: 'v1.1.0' }],
        }),
      ],
      components: {
        // Starlight's two loudest tells: its header lockup and its page
        // title block. Both are replaced so the site reads as catena's.
        //
        // The versions plugin also wants PageTitle (for its outdated-version
        // notice), and a config override wins over a plugin's. So Header and
        // PageTitle render the plugin's components themselves — see both
        // files. Without that the notice would be silently dropped and a
        // reader on an archived page would never be told.
        Header: './src/components/Header.astro',
        PageTitle: './src/components/PageTitle.astro',
        // Starlight renders ThemeSelect in the mobile menu footer as well
        // as in its header, and the versions plugin claims that slot for
        // its native <select>. Claiming it first keeps the OS dropdown off
        // the site entirely; the header already carries both the theme
        // control and the version switcher at every width.
        ThemeSelect: './src/components/ThemeToggle.astro',
        // Starlight's social links cannot carry a target, and a link out
        // to the repository should not replace the docs you are reading.
        SocialIcons: './src/components/SocialIcons.astro',
      },
      social: [{ icon: 'github', label: 'GitHub', href: site.repoUrl }],
      editLink: { baseUrl: `${site.repoUrl}/edit/main/` },
      lastUpdated: true,
      pagination: true,
      head: [
        { tag: 'link', attrs: { rel: 'preconnect', href: 'https://fonts.googleapis.com' } },
        {
          tag: 'link',
          attrs: { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: true },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'stylesheet',
            href:
              'https://fonts.googleapis.com/css2?' +
              'family=Fraunces:opsz,wght@9..144,500;9..144,600&' +
              'family=Inter:wght@400;500;600&' +
              'family=JetBrains+Mono:wght@400;500&display=swap',
          },
        },
        {
          tag: 'meta',
          attrs: { name: 'go-import', content: `${site.module} git ${site.repoUrl}` },
        },
      ],
      sidebar: [
        { label: 'Guides', items: [{ autogenerate: { directory: 'guides' } }] },
        { label: 'Operators', items: [{ autogenerate: { directory: 'operators' } }] },
        // The runnable programs answer "how do I build a log pipeline",
        // which is a different question from "what does DedupeBy do" —
        // hence a separate section from the operator reference.
        { label: 'Recipes', items: [{ autogenerate: { directory: 'examples' } }] },
        {
          label: 'Reference',
          items: [
            { slug: 'reference/versioning' },
            { slug: 'reference/changelog' },
            {
              label: 'API on pkg.go.dev',
              link: `https://pkg.go.dev/${site.module}`,
              attrs: { target: '_blank', rel: 'noopener noreferrer' },
            },
          ],
        },
      ],
    }),
  ],
})
