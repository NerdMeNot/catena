module github.com/NerdMeNot/catena

go 1.27

// Withdrawn versions. The v0.x tags were development checkpoints, and
// v1.0.0 was published from a tree this repository no longer contains —
// module proxies cache permanently, so both are retracted here rather
// than deleted. Use v1.1.0 or later.
retract (
	[v0.1.0, v0.3.0]
	v1.0.0
)

require pgregory.net/rapid v1.3.0
