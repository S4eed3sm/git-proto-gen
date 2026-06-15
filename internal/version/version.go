// Package version exposes the build version of git-proto-gen.
package version

// Version is the build version. It is overridden at link time via
//
//	-ldflags "-X github.com/S4eed3sm/git-proto-gen/internal/version.Version=<v>"
//
// and defaults to "dev" for `go run`/`go build` without ldflags.
var Version = "dev"
