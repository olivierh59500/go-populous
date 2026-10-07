// Command import-assets restores local game resources from an original ADF.
package main

import (
	"flag"
	"fmt"
	"os"

	"go-populous/internal/assetimport"
)

type diskPaths []string

func (p *diskPaths) String() string         { return fmt.Sprint([]string(*p)) }
func (p *diskPaths) Set(value string) error { *p = append(*p, value); return nil }

func main() {
	var disks diskPaths
	flag.Var(&disks, "adf", "user-supplied original Populous ADF; repeat for several disks")
	output := flag.String("output", "assets/amiga", "local game-resource directory")
	dryRun := flag.Bool("dry-run", false, "validate the disks and destination without writing files")
	verify := flag.Bool("verify", false, "check an existing resource directory without importing")
	flag.Parse()
	if flag.NArg() != 0 {
		fail(fmt.Errorf("use -adf for each original disk image"))
	}
	if *verify {
		if len(disks) != 0 || *dryRun {
			fail(fmt.Errorf("-verify cannot be combined with -adf or -dry-run"))
		}
		if err := assetimport.Validate(os.DirFS(*output)); err != nil {
			fail(err)
		}
		fmt.Printf("Verified %d original game resources in %s.\n", len(assetimport.Files), *output)
		return
	}
	count, err := assetimport.Import(assetimport.Config{ADFs: disks, Output: *output, DryRun: *dryRun})
	if err != nil {
		fail(err)
	}
	if *dryRun {
		fmt.Printf("Validated %d original game resources; no files were written.\n", count)
		return
	}
	fmt.Printf("Prepared %d original game resources in %s. Only local build inputs were imported.\n", count, *output)
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
