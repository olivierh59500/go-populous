package game

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestNetworkBuildIDUsesReleaseFingerprint(t *testing.T) {
	fingerprint := strings.Repeat("a", 64)
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v9.9.9"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: strings.Repeat("b", 40)},
			{Key: "vcs.modified", Value: "true"},
		},
	}

	got := networkBuildID(fingerprint, info, true)
	want := networkCompatibilityID + "-" + fingerprint
	if got != want {
		t.Fatalf("networkBuildID() = %q, want %q", got, want)
	}
}

func TestNetworkBuildIDWithoutReleaseFingerprintUsesVCSMetadata(t *testing.T) {
	revision := strings.Repeat("c", 40)
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: revision},
			{Key: "vcs.modified", Value: "true"},
		},
	}

	got := networkBuildID("", info, true)
	want := networkCompatibilityID + "-v1.2.3-" + revision[:12] + "-dirty"
	if got != want {
		t.Fatalf("networkBuildID() = %q, want %q", got, want)
	}
}

func TestNetworkBuildIDWithoutBuildInfoUsesCompatibilityID(t *testing.T) {
	if got := networkBuildID("", nil, false); got != networkCompatibilityID {
		t.Fatalf("networkBuildID() = %q, want %q", got, networkCompatibilityID)
	}
}

func TestNetworkReleaseFingerprintIsSafeAndBounded(t *testing.T) {
	for _, fingerprint := range []string{
		strings.Repeat("x", 129),
		"contains whitespace and a control\x01character",
	} {
		got := networkBuildID(fingerprint, nil, false)
		if !strings.HasPrefix(got, networkCompatibilityID+"-") {
			t.Fatalf("networkBuildID(%q) = %q, missing compatibility prefix", fingerprint, got)
		}
		if len(got) > 128 {
			t.Fatalf("networkBuildID(%q) has length %d, want at most 128", fingerprint, len(got))
		}
		if strings.ContainsAny(got, " \t\r\n\x01") {
			t.Fatalf("networkBuildID(%q) produced unsafe ID %q", fingerprint, got)
		}
	}
}
