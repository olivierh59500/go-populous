package assetimport

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go-populous/internal/amiga"
)

// Config describes a local import. DryRun validates inputs and existing output
// without creating directories or files. Several disks may supply one resource
// set; a conflicting resource is always rejected.
type Config struct {
	ADFs   []string
	Output string
	DryRun bool
}

// Import validates the complete resource set before touching the destination.
// Matching existing files are reused. New files are published atomically without
// replacing existing paths, and symbolic links are never used as output paths.
func Import(config Config) (int, error) {
	return importFiles(config, Files)
}

func importFiles(config Config, specs []Fingerprint) (int, error) {
	if len(config.ADFs) == 0 {
		return 0, fmt.Errorf("supply an original Populous disk image with -adf")
	}
	if config.Output == "" {
		return 0, fmt.Errorf("an output directory is required")
	}
	wanted := make(map[string]Fingerprint, len(specs))
	for _, spec := range specs {
		if !fs.ValidPath(spec.Name) || path.Base(spec.Name) != spec.Name || spec.Name != strings.ToLower(spec.Name) {
			return 0, fmt.Errorf("invalid resource name %q", spec.Name)
		}
		wanted[spec.Name] = spec
	}
	contents := make(map[string][]byte, len(specs))
	for _, input := range config.ADFs {
		raw, err := os.ReadFile(input)
		if err != nil {
			return 0, fmt.Errorf("read ADF %q: %w", input, err)
		}
		disk, err := amiga.ParseDisk(raw)
		if err != nil {
			return 0, fmt.Errorf("ADF %q: %w", input, err)
		}
		for _, entry := range disk.Entries() {
			if entry.Directory {
				continue
			}
			name := strings.ToLower(path.Base(entry.Path))
			spec, required := wanted[name]
			if !required {
				continue
			}
			data, err := disk.ReadFile(entry.Path)
			if err != nil {
				return 0, fmt.Errorf("ADF %q: %w", input, err)
			}
			if err := check(spec, data); err != nil {
				return 0, fmt.Errorf("ADF %q: %w", input, err)
			}
			if previous, ok := contents[name]; ok && !bytes.Equal(previous, data) {
				return 0, fmt.Errorf("conflicting input for %s", name)
			}
			contents[name] = data
		}
	}
	var missing []string
	for _, spec := range specs {
		if _, ok := contents[spec.Name]; !ok {
			missing = append(missing, spec.Name)
		}
	}
	if len(missing) != 0 {
		return 0, fmt.Errorf("missing original game resources %v; supply a compatible original Populous ADF (see docs/ASSET_SETUP.md)", missing)
	}
	root, err := outputRoot(config.Output, false)
	if err != nil {
		return 0, err
	}
	if root != nil {
		for _, spec := range specs {
			if _, err := existingMatches(root, spec.Name, contents[spec.Name]); err != nil {
				root.Close()
				return 0, err
			}
		}
		root.Close()
	}
	if config.DryRun {
		return len(specs), nil
	}
	root, err = outputRoot(config.Output, true)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	// Recheck through the opened directory handle before staging any resource.
	for _, spec := range specs {
		if _, err := existingMatches(root, spec.Name, contents[spec.Name]); err != nil {
			return 0, err
		}
	}
	staged := make(map[string]string, len(specs))
	defer func() {
		for _, temp := range staged {
			_ = root.Remove(temp)
		}
	}()
	for _, spec := range specs {
		exists, err := existingMatches(root, spec.Name, contents[spec.Name])
		if err != nil {
			return 0, err
		}
		if exists {
			continue
		}
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return 0, err
		}
		temp := fmt.Sprintf(".populous-import-%x", nonce)
		file, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return 0, err
		}
		staged[spec.Name] = temp
		_, writeErr := file.Write(contents[spec.Name])
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil {
			return 0, writeErr
		}
		if closeErr != nil {
			return 0, closeErr
		}
	}
	created := make([]string, 0, len(staged))
	rollback := func() {
		for _, name := range created {
			_ = root.Remove(name)
		}
	}
	for _, spec := range specs {
		temp, needsWrite := staged[spec.Name]
		if !needsWrite {
			continue
		}
		// A hard link publishes the fully written file in one operation. Unlike
		// Rename, it cannot overwrite a file created by another importer meanwhile.
		if err := root.Link(temp, spec.Name); err != nil {
			if exists, matchErr := existingMatches(root, spec.Name, contents[spec.Name]); matchErr == nil && exists {
				continue
			}
			rollback()
			return 0, fmt.Errorf("publish %s without replacing an existing path: %w", spec.Name, err)
		}
		created = append(created, spec.Name)
	}
	return len(specs), nil
}

func existingMatches(root *os.Root, name string, expected []byte) (bool, error) {
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("output %q is not a regular file; symbolic links are not allowed", name)
	}
	data, err := root.ReadFile(name)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(data, expected) {
		return false, fmt.Errorf("existing output %q differs; choose another directory instead of overwriting it", name)
	}
	return true, nil
}

// outputRoot walks every directory component through an opened root, rejecting
// symlinks instead of following them. The returned handle confines later writes
// even if a parent directory is renamed while the import runs.
func outputRoot(destination string, create bool) (*os.Root, error) {
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return nil, err
	}
	volume := filepath.VolumeName(absolute)
	root, err := os.OpenRoot(volume + string(os.PathSeparator))
	if err != nil {
		return nil, err
	}
	components := strings.Split(strings.TrimPrefix(absolute[len(volume):], string(os.PathSeparator)), string(os.PathSeparator))
	for _, component := range components {
		if component == "" {
			continue
		}
		info, err := root.Lstat(component)
		if os.IsNotExist(err) {
			if !create {
				root.Close()
				return nil, nil
			}
			if err = root.Mkdir(component, 0755); err != nil && !os.IsExist(err) {
				root.Close()
				return nil, err
			}
			info, err = root.Lstat(component)
		}
		if err != nil {
			root.Close()
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			root.Close()
			return nil, fmt.Errorf("output directory component %q is not a real directory; symbolic links are not allowed", component)
		}
		next, err := root.OpenRoot(component)
		root.Close()
		if err != nil {
			return nil, err
		}
		root = next
	}
	return root, nil
}

func check(spec Fingerprint, data []byte) error {
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if len(data) != spec.Size || digest != spec.SHA256 {
		return fmt.Errorf("unsupported or damaged %s: got %d bytes, SHA-256 %s; expected %d bytes, SHA-256 %s", spec.Name, len(data), digest, spec.Size, spec.SHA256)
	}
	return nil
}

// Validate checks a prepared installation without importing or executing files.
func Validate(files fs.FS) error {
	for _, spec := range Files {
		data, err := fs.ReadFile(files, spec.Name)
		if err != nil {
			return fmt.Errorf("missing resource %s: %w; see docs/ASSET_SETUP.md", spec.Name, err)
		}
		if err := check(spec, data); err != nil {
			return err
		}
	}
	return nil
}
