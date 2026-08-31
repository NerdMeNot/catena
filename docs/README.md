# catena documentation

Start here, read in roughly this order:

- **[Getting started](01-getting-started.md)** — install, first pipeline, the
  five-minute tour.
- **[Concepts](02-concepts.md)** — the four types, laziness, the single-pass
  contract, and the package-wide rules that make every operator predictable.
- **[Error handling](03-error-handling.md)** — `Try`, the producer patterns for
  files and databases, and choosing an error policy at the consumer.
- **[Operators](04-operators.md)** — the catalog: every operator with its
  memory class, drain class, and the notes that matter.
- **[Performance](05-performance.md)** — what the iterator protocol costs, what
  catena adds (nothing, measured), and when fusion wins outright.
- **[Releasing](06-releasing.md)** — how a release is cut and verified.

**[Operator reference](operators/)** — all 182 operators across 14 pages,
each with its signature, its memory and termination behaviour, and a
worked example that `go test` runs and verifies. Generated from the
library's own doc comments and its Example functions, so it cannot drift
from the code.

API reference with full signatures:
[pkg.go.dev/github.com/NerdMeNot/catena](https://pkg.go.dev/github.com/NerdMeNot/catena).

The design rationale — why each decision was made and what was rejected —
lives in [SPEC.md](../SPEC.md) at the repository root. When a doc here and
the spec disagree, the spec wins and the doc has a bug; please report it.
