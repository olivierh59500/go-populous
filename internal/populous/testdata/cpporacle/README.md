# Optional original C++ oracle

Run from the repository root:

```sh
go test -tags cpporacle ./internal/populous -run '^TestCPPOracle' -count=1 -v
```

This needs `c++` on PATH and the original project in
`previous/DCPopulous-master`. Missing prerequisites produce an explicit test
skip. The ordinary Go suite does not require either prerequisite.

The Go test extracts eight original function bodies verbatim, preserving source
locations and logging source SHA-256 hashes. Only temporary files contain these
bodies; this directory contains newly written portable declarations and test
fixtures, not a copy of the original game implementation. Compilation and
execution have timeouts, and temporary files are removed by the test framework.

Coverage: terrain primitives for all 495 levels with zero/four prior RNG draws,
complete generated landscape arrays and final RNG for those levels, nonfatal
combat rounds, and knight reinforcements into a defending friendly town.

The test does not establish full-game equivalence. Unused graphics calls are
stubbed, and unimplemented victory/death callbacks abort rather than silently
pass. Altitude padding makes the original speculative edge reads safe without
rewriting its algorithms. A separate ordinary Go regression test retains the
495-world terrain fingerprint even when the original sources are unavailable.
