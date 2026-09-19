package mobileui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// BuildScope selects the local construction-presence rule. It changes neither
// the simulation state nor the original world's terrain and mana restrictions.
type BuildScope int

const (
	BuildScopeClassic BuildScope = iota
	BuildScopeVisible
)

// Preferences are presentation and local-input settings, stored separately
// from a saved world. Multiplayer can enforce its own construction scope.
type Preferences struct {
	Wide       bool       `json:"wide"`
	ShowPad    bool       `json:"show_pad"`
	BuildScope BuildScope `json:"build_scope"`
}

func DefaultPreferences() Preferences {
	return Preferences{Wide: true, ShowPad: false, BuildScope: BuildScopeClassic}
}

func validBuildScope(scope BuildScope) bool {
	return scope == BuildScopeClassic || scope == BuildScopeVisible
}

// LoadPreferences merges missing fields with defaults, allowing settings from
// older versions to remain useful. On failure it returns defaults and an error;
// a missing file is distinguishable with errors.Is(err, os.ErrNotExist).
func LoadPreferences(path string) (Preferences, error) {
	defaults := DefaultPreferences()
	file, err := os.Open(path)
	if err != nil {
		return defaults, fmt.Errorf("open mobile preferences: %w", err)
	}
	defer file.Close()

	preferences := defaults
	decoder := json.NewDecoder(io.LimitReader(file, 64*1024))
	if err := decoder.Decode(&preferences); err != nil {
		return defaults, fmt.Errorf("decode mobile preferences: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return defaults, fmt.Errorf("decode mobile preferences: %w", err)
	}
	if !validBuildScope(preferences.BuildScope) {
		return defaults, fmt.Errorf("invalid mobile construction scope: %d", preferences.BuildScope)
	}
	return preferences, nil
}

// SavePreferences replaces only the chosen file, using a private temporary
// file and an atomic rename in the same directory. The directory must exist.
func SavePreferences(path string, preferences Preferences) error {
	if !validBuildScope(preferences.BuildScope) {
		return fmt.Errorf("invalid mobile construction scope: %d", preferences.BuildScope)
	}
	if path == "" {
		return errors.New("mobile preferences path is empty")
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create mobile preferences: %w", err)
	}
	temporaryPath := file.Name()
	defer os.Remove(temporaryPath)
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(preferences); err != nil {
		return fmt.Errorf("write mobile preferences: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync mobile preferences: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close mobile preferences: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace mobile preferences: %w", err)
	}
	return nil
}
