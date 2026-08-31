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
// The version shown in the header comes from `version.go`, through the
// sync script — there is no version string written down twice. Today the
// site documents one release, so it needs no version *switcher*; when a
// second release exists, add `starlight-versions` (already a dependency)
// with the previous release as an archived version. The procedure is in
// docs/06-releasing.md, under "Versioning the website".
import { defineConfig } from 'astro/config'
import starlight from '@astrojs/starlight'

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
      components: {
        // Starlight's two loudest tells: its header lockup and its page
        // title block. Both are replaced so the site reads as catena's.
        Header: './src/components/Header.astro',
        PageTitle: './src/components/PageTitle.astro',
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
              attrs: { target: '_blank' },
            },
          ],
        },
      ],
    }),
  ],
})
