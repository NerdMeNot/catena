// The packages the analyzers are tested on. They import the real catena,
// from this repository, so a check can never pass against a stand-in that
// has drifted from the library.
module lintdata

go 1.27

require github.com/NerdMeNot/catena v1.5.0

replace github.com/NerdMeNot/catena => ../..
