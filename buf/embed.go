// Package buf embeds the default buf configuration templates so they ship
// inside the binary. Override files supplied via --buf-configs take precedence
// at render time; these are the fallback defaults.
package buf

import "embed"

// FS holds the embedded buf.yaml, buf.gen.go.yaml and buf.gen.js.yaml.
//
//go:embed *.yaml
var FS embed.FS
