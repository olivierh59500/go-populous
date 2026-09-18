// Package assets contains the data files that must be available when Populous
// is built for a platform without a repository working directory, notably
// Android.
package assets

import "embed"

// Files contains the canonical Amiga data used by the game. The two extracted
// PNGs are required because lord.pic and load.pic contain additional data and
// their checked-in reference images cannot be reproduced by the plain planar
// screen decoder. The other extracted PNGs duplicate screens that can be
// decoded exactly and are deliberately not embedded.
//
//go:embed amiga/demo.pic amiga/gmusic1 amiga/gwords amiga/land0 amiga/land1 amiga/land2 amiga/land3 amiga/level.dat amiga/load.pic amiga/lord.pic amiga/mouths.pic amiga/qaz.pic amiga/spr_320.dat amiga/sprites0.dat extracted-images/load.pic.png extracted-images/lord.pic.png
var Files embed.FS
