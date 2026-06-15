package config

import "testing"

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"local only", Config{LocalPath: "proto", Languages: []string{"go"}}, false},
		{"repo only", Config{Repos: []string{"github.com/o/r//proto"}, Languages: []string{"go", "js"}}, false},
		{"no source", Config{Languages: []string{"go"}}, true},
		{"no language", Config{LocalPath: "proto"}, true},
		{"bad language", Config{LocalPath: "proto", Languages: []string{"rust"}}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestParseMergesDeprecatedRepoFlags(t *testing.T) {
	cfg, err := Parse([]string{
		"--repo", "github.com/o/r//proto",
		"--private-repo", "github.com/o/priv//proto",
		"--public-repo", "github.com/o/pub//proto",
		"--lang", "go",
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	want := []string{
		"github.com/o/r//proto",
		"github.com/o/priv//proto",
		"github.com/o/pub//proto",
	}
	if len(cfg.Repos) != len(want) {
		t.Fatalf("Repos = %v, want %v", cfg.Repos, want)
	}
	for i, w := range want {
		if cfg.Repos[i] != w {
			t.Errorf("Repos[%d] = %q, want %q", i, cfg.Repos[i], w)
		}
	}
}

func TestParseInvalidReturnsError(t *testing.T) {
	if _, err := Parse([]string{"--lang", "go"}); err == nil {
		t.Error("expected error when no source provided")
	}
}
