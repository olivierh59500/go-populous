// Package assets contains the data files that must be available when Populous
// is built for a platform without a repository working directory, notably
// Android.
package assets

import "embed"

// Files contains locally imported resources. The directories also contain
// tracked text placeholders so the source builds before resources are imported.
// Original data and generated images are deliberately excluded from Git.
//
//go:embed amiga extracted-images
var Files embed.FS
