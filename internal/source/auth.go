package source

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kind is the kind of credential resolved for a host.
type Kind int

const (
	KindAnonymous Kind = iota // public HTTPS, no credential
	KindToken                 // HTTPS with a personal access token
	KindSSH                   // SSH key / agent
)

func (k Kind) String() string {
	switch k {
	case KindToken:
		return "token"
	case KindSSH:
		return "ssh"
	default:
		return "anonymous"
	}
}

// Credential is the resolved authentication for a host. The token is unexported
// and redacted by LogValue so it never reaches logs.
type Credential struct {
	Kind  Kind
	token string
}

// Token returns the secret. Callers must keep it out of process arguments and
// logs; it is injected into git via a credential helper, not the command line.
func (c *Credential) Token() string { return c.token }

// LogValue implements slog.LogValuer, redacting the secret.
func (c *Credential) LogValue() slog.Value {
	if c == nil {
		return slog.StringValue("none")
	}
	if c.Kind == KindToken {
		return slog.StringValue("token(redacted)")
	}
	return slog.StringValue(c.Kind.String())
}

// AuthResolver maps a host to the credential to use for it. Credentials are
// per-host (not per-source) because every repo on a host shares them.
type AuthResolver interface {
	Resolve(host string) (*Credential, error)
}

// EnvAuthResolver resolves credentials, in order: a host-specific environment
// variable, the legacy --token (github.com only), an --auth-config entry, an
// available SSH key, then anonymous.
type EnvAuthResolver struct {
	legacyToken  string            // from --token, applies to github.com
	hostTokens   map[string]string // from --auth-config, host -> token
	lookupEnv    func(string) (string, bool)
	sshAvailable func() bool
}

// NewEnvAuthResolver builds a resolver from the legacy --token and an optional
// --auth-config file path (empty to skip).
func NewEnvAuthResolver(legacyToken, authConfigPath string) (*EnvAuthResolver, error) {
	hostTokens := map[string]string{}
	if authConfigPath != "" {
		loaded, err := loadAuthConfig(authConfigPath)
		if err != nil {
			return nil, err
		}
		hostTokens = loaded
	}
	return &EnvAuthResolver{
		legacyToken:  legacyToken,
		hostTokens:   hostTokens,
		lookupEnv:    os.LookupEnv,
		sshAvailable: sshKeysPresent,
	}, nil
}

// Resolve implements AuthResolver.
func (r *EnvAuthResolver) Resolve(host string) (*Credential, error) {
	if v, ok := r.lookupEnv(EnvVarForHost(host)); ok && v != "" {
		return &Credential{Kind: KindToken, token: v}, nil
	}
	if host == "github.com" && r.legacyToken != "" {
		return &Credential{Kind: KindToken, token: r.legacyToken}, nil
	}
	if t, ok := r.hostTokens[host]; ok && t != "" {
		return &Credential{Kind: KindToken, token: t}, nil
	}
	if r.sshAvailable != nil && r.sshAvailable() {
		return &Credential{Kind: KindSSH}, nil
	}
	return &Credential{Kind: KindAnonymous}, nil
}

// EnvVarForHost returns the per-host token environment variable name, e.g.
// "gitlab.example.com" -> "GIT_PROTO_GEN_TOKEN_GITLAB_EXAMPLE_COM".
func EnvVarForHost(host string) string {
	var b strings.Builder
	b.WriteString("GIT_PROTO_GEN_TOKEN_")
	for _, r := range strings.ToUpper(host) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// authConfig is the on-disk --auth-config schema. tokenEnv is preferred over an
// inline token so secrets are not stored on disk.
type authConfig struct {
	Hosts map[string]struct {
		Token    string `yaml:"token"`
		TokenEnv string `yaml:"tokenEnv"`
	} `yaml:"hosts"`
}

func loadAuthConfig(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read auth config %q: %w", path, err)
	}
	var cfg authConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse auth config %q: %w", path, err)
	}
	tokens := make(map[string]string, len(cfg.Hosts))
	for host, entry := range cfg.Hosts {
		switch {
		case entry.TokenEnv != "":
			if v, ok := os.LookupEnv(entry.TokenEnv); ok {
				tokens[host] = v
			}
		case entry.Token != "":
			tokens[host] = entry.Token
		}
	}
	return tokens, nil
}

// sshKeysPresent reports whether a usable SSH key exists in ~/.ssh.
func sshKeysPresent() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	for _, name := range []string{"id_rsa", "id_ed25519", "id_ecdsa"} {
		if _, err := os.Stat(filepath.Join(home, ".ssh", name)); err == nil {
			return true
		}
	}
	return false
}
