package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/S4eed3sm/git-proto-gen/internal/config"
	"github.com/S4eed3sm/git-proto-gen/internal/generate"
	"github.com/S4eed3sm/git-proto-gen/internal/source"
)

type fakeSource struct {
	repoName string
	stage    func(dst string) error
}

func (s *fakeSource) Fetch(_ context.Context, dst string) (*source.Result, error) {
	if s.stage != nil {
		if err := s.stage(dst); err != nil {
			return nil, err
		}
	}
	return &source.Result{RepoName: s.repoName, LocalDir: dst}, nil
}

type fakeBuilder struct {
	localCalls  int
	remoteSpecs []*source.Spec
}

func (b *fakeBuilder) Local(string) source.Source {
	b.localCalls++
	return &fakeSource{stage: func(dst string) error {
		return os.WriteFile(filepath.Join(dst, "local.proto"), []byte("syntax=\"proto3\";"), 0o644)
	}}
}

func (b *fakeBuilder) Remote(spec *source.Spec) (source.Source, error) {
	b.remoteSpecs = append(b.remoteSpecs, spec)
	return &fakeSource{repoName: spec.RepoName()}, nil
}

type fakeGenerator struct {
	jobs   []*generate.Job
	wsDir  string
	outDir string
}

func (g *fakeGenerator) Generate(_ context.Context, wsDir, outDir string, jobs []*generate.Job) error {
	g.wsDir, g.outDir, g.jobs = wsDir, outDir, jobs
	// Emit a sentinel so the copy-to-output step has something to copy.
	return os.WriteFile(filepath.Join(outDir, "generated.txt"), []byte("ok"), 0o644)
}

func TestAppRun(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	cfg := &config.Config{
		LocalPath:  t.TempDir(),
		Repos:      []string{"gitlab.example.com/group/sub/proj//proto@main"},
		Languages:  []string{"go", "js"},
		OutputPath: out,
	}
	builder := &fakeBuilder{}
	gen := &fakeGenerator{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := New(builder, gen, log).Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if builder.localCalls != 1 {
		t.Errorf("local source built %d times, want 1", builder.localCalls)
	}
	if len(builder.remoteSpecs) != 1 || builder.remoteSpecs[0].RepoPath != "group/sub/proj" {
		t.Errorf("remote specs = %+v, want one for group/sub/proj", builder.remoteSpecs)
	}

	if len(gen.jobs) != 2 {
		t.Fatalf("jobs = %d, want 2 (go, js)", len(gen.jobs))
	}
	if gen.jobs[0].Lang != "go" || gen.jobs[1].Lang != "js" {
		t.Errorf("job langs = %q,%q want go,js", gen.jobs[0].Lang, gen.jobs[1].Lang)
	}

	// Generated output copied to the resolved --output directory.
	if _, err := os.Stat(filepath.Join(out, "generated.txt")); err != nil {
		t.Errorf("expected generated output at %q: %v", out, err)
	}
}

func TestAppRunBadSpecFails(t *testing.T) {
	cfg := &config.Config{
		Repos:      []string{"gitlab.example.com/group/sub/proj/proto"}, // no // separator
		Languages:  []string{"go"},
		OutputPath: filepath.Join(t.TempDir(), "out"),
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := New(&fakeBuilder{}, &fakeGenerator{}, log).Run(context.Background(), cfg); err == nil {
		t.Error("expected error for ambiguous non-github spec without '//'")
	}
}
