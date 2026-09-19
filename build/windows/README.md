# Windows executable resources

`winres.json` is the source for the icon, application manifest, and Windows
version information embedded in every release executable. The build invokes
the pinned `github.com/tc-hib/go-winres@v0.3.3` tool and generates temporary
architecture-specific `.syso` files next to `cmd/populous/main.go`.
`go-winres` and its `winres` library use the permissive 0BSD license and are
build-time tools only; neither is linked into the released game.

The icon reuses the tracked Android launcher image, itself generated from
`assets/extracted-images/load.pic.png`. This keeps the presentation consistent
without introducing a second master artwork. The original Populous artwork and
trademarks are not covered by the source-code GPL; see the repository README
before redistributing binaries.

The manifest runs the game as the current user, declares Windows 10
compatibility, per-monitor-v2 DPI awareness, high-resolution scrolling, and
long-path awareness. It never requests administrator privileges.

No company or copyright owner is placed in `VERSIONINFO`: the repository does
not establish one for this port, and the original game's rights remain with
their respective holders.
