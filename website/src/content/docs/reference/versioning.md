---
title: "Versioning"
description: "These docs describe catena v1.3.0. What the version guarantees, how to pin it, and how the docs track releases."
---

## What these docs describe

**This site documents catena v1.3.0** — the version shown in the header,
and the one `go get github.com/NerdMeNot/catena` resolves to. That number is read from
the library's own `Version` constant when the site is built, so the docs
and the code cannot disagree about which release is being described; CI
fails the build if they drift.

## What v1 guarantees

catena follows [semantic versioning](https://semver.org). `v1.1.0`,
the first supported release, froze
the API: within v1, operators keep their names, signatures, and documented
behaviour — including the contracts that are easy to depend on without
noticing, such as which operators buffer, which terminate early, and what
each does with errors. Those are specified per operator and enforced by the
conformance suite, so they are part of the compatibility promise rather
than incidental behaviour.

Anything that would break a working program waits for v2.

## Pinning

```sh
go get github.com/NerdMeNot/catena@v1.3.0
```

Go modules pin by default — your `go.mod` records the exact version and
`go.sum` its checksum, so a build is reproducible without further effort.

### Retracted versions

The `v0.x` development checkpoints, and `v1.0.0` — published briefly and
withdrawn — are **retracted** in `go.mod`. Module proxies cache versions
permanently, so those tags may still appear in `go list -m -versions`
output; the retraction marks them as withdrawn, and `go get` will not
select one. Use v1.1.0 or later.

## How the docs track releases

Today catena has one released version, so this site documents it directly
and needs no version switcher. When a second release exists, the previous
one is archived at `/1-3/…` and a
version selector appears in the header
— the machinery ([starlight-versions](https://starlight-versions.vercel.app))
is already a dependency, and the procedure is written down in the
repository's releasing guide.

Until then, the rule that matters is the one above: the version in the
header is read from the source, not typed by hand.
