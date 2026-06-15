package source

import "testing"

func TestParseSpec(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    Spec
		wantErr bool
	}{
		{
			name: "github with subdir and ref",
			raw:  "github.com/S4eed3sm/public-test-proto//proto@dev",
			want: Spec{Host: "github.com", RepoPath: "S4eed3sm/public-test-proto", Subdir: "proto", Ref: "dev"},
		},
		{
			name: "nested gitlab groups",
			raw:  "gitlab.example.com/group/subgroup/project//proto/dir@main",
			want: Spec{Host: "gitlab.example.com", RepoPath: "group/subgroup/project", Subdir: "proto/dir", Ref: "main"},
		},
		{
			name: "explicit ssh scheme",
			raw:  "ssh://git@gitlab.example.com/g/s/p.git//proto@v1.2.0",
			want: Spec{Scheme: "ssh", Host: "gitlab.example.com", RepoPath: "g/s/p", Subdir: "proto", Ref: "v1.2.0"},
		},
		{
			name: "scp form with subdir and ref",
			raw:  "git@github.com:S4eed3sm/private-test-proto.git//proto@main",
			want: Spec{Scheme: "ssh", Host: "github.com", RepoPath: "S4eed3sm/private-test-proto", Subdir: "proto", Ref: "main"},
		},
		{
			name: "https scheme no ref",
			raw:  "https://github.com/o/r//proto",
			want: Spec{Scheme: "https", Host: "github.com", RepoPath: "o/r", Subdir: "proto"},
		},
		{
			name: "legacy github positional with subdir",
			raw:  "github.com/o/r/proto@dev",
			want: Spec{Host: "github.com", RepoPath: "o/r", Subdir: "proto", Ref: "dev", Legacy: true},
		},
		{
			name: "legacy github positional nested subdir",
			raw:  "github.com/o/r/proto/sub",
			want: Spec{Host: "github.com", RepoPath: "o/r", Subdir: "proto/sub", Legacy: true},
		},
		{
			name: "legacy github repo root only",
			raw:  "github.com/o/r",
			want: Spec{Host: "github.com", RepoPath: "o/r", Legacy: true},
		},
		{
			name: "branch containing slash",
			raw:  "github.com/o/r//proto@feature/x",
			want: Spec{Host: "github.com", RepoPath: "o/r", Subdir: "proto", Ref: "feature/x"},
		},
		{
			name:    "non-github host without separator is rejected",
			raw:     "gitlab.example.com/group/sub/project/proto",
			wantErr: true,
		},
		{
			name:    "path traversal in subdir is rejected",
			raw:     "github.com/o/r//../../etc@main",
			wantErr: true,
		},
		{
			name:    "empty spec",
			raw:     "",
			wantErr: true,
		},
		{
			name:    "missing repo path",
			raw:     "github.com",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSpec(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseSpec(%q) = %+v, want error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSpec(%q) unexpected error: %v", tc.raw, err)
			}
			if got.Scheme != tc.want.Scheme || got.Host != tc.want.Host ||
				got.RepoPath != tc.want.RepoPath || got.Subdir != tc.want.Subdir ||
				got.Ref != tc.want.Ref || got.Legacy != tc.want.Legacy {
				t.Errorf("ParseSpec(%q)\n got  %+v\n want %+v", tc.raw, *got, tc.want)
			}
		})
	}
}

func TestSpecRepoNameAndCloneURL(t *testing.T) {
	s := &Spec{Host: "gitlab.example.com", RepoPath: "group/sub/project"}
	if got := s.RepoName(); got != "project" {
		t.Errorf("RepoName() = %q, want %q", got, "project")
	}
	if got := s.cloneURL(false); got != "https://gitlab.example.com/group/sub/project.git" {
		t.Errorf("cloneURL(https) = %q", got)
	}
	if got := s.cloneURL(true); got != "git@gitlab.example.com:group/sub/project.git" {
		t.Errorf("cloneURL(ssh) = %q", got)
	}

	explicit := &Spec{Scheme: "ssh", Host: "h", RepoPath: "o/r"}
	if got := explicit.cloneURL(false); got != "git@h:o/r.git" {
		t.Errorf("explicit ssh cloneURL = %q, want scp form", got)
	}
}
