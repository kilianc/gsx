package main

import (
	"runtime"
	"runtime/debug"
	"testing"
)

// The version line is what someone pastes into a bug report, so its shape is
// worth pinning: the release case, the two ways a checkout build identifies
// itself, and a binary whose build info was stripped.
func TestVersionLine(t *testing.T) {
	platform := " go1.22.5 " + runtime.GOOS + "/" + runtime.GOARCH

	vcs := func(rev, modified string) []debug.BuildSetting {
		return []debug.BuildSetting{
			{Key: "-compiler", Value: "gc"},
			{Key: "vcs", Value: "git"},
			{Key: "vcs.revision", Value: rev},
			{Key: "vcs.modified", Value: modified},
		}
	}

	tests := []struct {
		name string
		bi   *debug.BuildInfo
		want string
	}{
		{
			name: "installed at a tag",
			bi: &debug.BuildInfo{
				GoVersion: "go1.22.5",
				Main:      debug.Module{Path: "github.com/kilianc/gsx", Version: "v0.2.0"},
			},
			want: "gsx v0.2.0" + platform,
		},
		{
			name: "pseudo-version",
			bi: &debug.BuildInfo{
				GoVersion: "go1.22.5",
				Main:      debug.Module{Version: "v0.2.1-0.20260812041536-b76ffb8b1ef5"},
			},
			want: "gsx v0.2.1-0.20260812041536-b76ffb8b1ef5" + platform,
		},
		{
			name: "checkout build carries the commit",
			bi: &debug.BuildInfo{
				GoVersion: "go1.22.5",
				Main:      debug.Module{Version: "(devel)"},
				Settings:  vcs("b76ffb8b1ef59f194e2f256ce2c092f14c1a0973", "false"),
			},
			want: "gsx (devel b76ffb8b1ef5)" + platform,
		},
		{
			name: "uncommitted changes are marked dirty",
			bi: &debug.BuildInfo{
				GoVersion: "go1.22.5",
				Main:      debug.Module{Version: "(devel)"},
				Settings:  vcs("b76ffb8b1ef59f194e2f256ce2c092f14c1a0973", "true"),
			},
			want: "gsx (devel b76ffb8b1ef5 dirty)" + platform,
		},
		{
			name: "no version and no vcs stamps",
			bi: &debug.BuildInfo{
				GoVersion: "go1.22.5",
				Main:      debug.Module{Version: ""},
			},
			want: "gsx (devel)" + platform,
		},
		{
			name: "build info stripped",
			bi:   nil,
			want: "gsx (unknown) " + runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionLine(tt.bi); got != tt.want {
				t.Errorf("versionLine()\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

// The version of the binary running the tests is whatever the go command
// stamped, so this only asserts that a real build produces a usable line
// rather than an empty or half-formed one.
func TestVersionLineFromThisBuild(t *testing.T) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("no build info in the test binary")
	}
	got := versionLine(bi)
	if want := "gsx "; len(got) <= len(want) || got[:len(want)] != want {
		t.Fatalf("versionLine() = %q, want it to start with %q", got, want)
	}
	t.Logf("this build: %s", got)
}
