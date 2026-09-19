package mobileui

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPreferencesLoadMergesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := os.WriteFile(path, []byte(`{"show_pad":true,"future_option":42}`), 0600); err != nil {
		t.Fatal(err)
	}
	preferences, err := LoadPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	want := DefaultPreferences()
	want.ShowPad = true
	if preferences != want {
		t.Fatalf("partial preferences = %+v, want %+v", preferences, want)
	}
}

func TestPreferencesLoadErrorsReturnDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if preferences, err := LoadPreferences(path); !errors.Is(err, os.ErrNotExist) || preferences != DefaultPreferences() {
		t.Fatalf("missing file = %+v, %v", preferences, err)
	}
	for _, content := range []string{`{`, `{"wide":false,"build_scope":9}`, `{"build_scope":-1}`, `{} {}`, `[]`, `{"wide":"yes"}`} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		preferences, err := LoadPreferences(path)
		if err == nil || preferences != DefaultPreferences() {
			t.Fatalf("invalid %q = %+v, %v", content, preferences, err)
		}
	}
}

func TestPreferencesAtomicReplacement(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "preferences.json")
	if err := os.WriteFile(path, []byte("old contents"), 0644); err != nil {
		t.Fatal(err)
	}
	want := Preferences{Wide: false, ShowPad: true, BuildScope: BuildScopeVisible}
	if err := SavePreferences(path, want); err != nil {
		t.Fatal(err)
	}
	preferences, err := LoadPreferences(path)
	if err != nil || preferences != want {
		t.Fatalf("round trip = %+v, %v; want %+v", preferences, err, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("permissions = %v, want 0600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "preferences.json" {
		t.Fatalf("leftover temporary file: %v, %v", entries, err)
	}
	invalid := want
	invalid.BuildScope = 20
	if err := SavePreferences(path, invalid); err == nil {
		t.Fatal("invalid scope saved")
	}
	if preferences, err = LoadPreferences(path); err != nil || preferences != want {
		t.Fatalf("failed save changed existing preferences: %+v, %v", preferences, err)
	}
}

func TestPreferencesFailedRenameCleansTemporaryFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "existing-directory")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := SavePreferences(path, DefaultPreferences()); err == nil {
		t.Fatal("replaced directory with file")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "existing-directory" || !entries[0].IsDir() {
		t.Fatalf("failed save changed directory or leaked temporary file: %v, %v", entries, err)
	}
	if err := SavePreferences("", DefaultPreferences()); err == nil {
		t.Fatal("empty path accepted")
	}
	if err := SavePreferences(filepath.Join(directory, "missing-parent", "prefs.json"), DefaultPreferences()); err == nil {
		t.Fatal("missing parent accepted")
	}
}
