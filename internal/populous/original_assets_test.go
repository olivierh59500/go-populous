package populous

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Original-data regression tests run when a local ADF has been imported. Unit
// tests based on synthetic data remain available in a resource-free checkout.
func requireOriginalResources(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join("../../assets/amiga", name)
		_, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			t.Skipf("original resource %s is not imported; see docs/ASSET_SETUP.md", name)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}
