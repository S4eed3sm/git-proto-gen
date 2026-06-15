package app

import "github.com/S4eed3sm/git-proto-gen/internal/source"

// defaultSourceBuilder builds real local and clone sources, resolving per-host
// credentials via the auth resolver and cloning via the git runner.
type defaultSourceBuilder struct {
	auth   source.AuthResolver
	runner source.GitRunner
}

// NewSourceBuilder returns the production SourceBuilder.
func NewSourceBuilder(auth source.AuthResolver, runner source.GitRunner) SourceBuilder {
	return &defaultSourceBuilder{auth: auth, runner: runner}
}

func (b *defaultSourceBuilder) Local(dir string) source.Source {
	return source.NewLocalSource(dir)
}

func (b *defaultSourceBuilder) Remote(spec *source.Spec) (source.Source, error) {
	cred, err := b.auth.Resolve(spec.Host)
	if err != nil {
		return nil, err
	}
	return source.NewCloneSource(spec, cred, b.runner), nil
}
