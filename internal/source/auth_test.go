package source

import "testing"

func TestEnvVarForHost(t *testing.T) {
	tests := map[string]string{
		"github.com":         "GIT_PROTO_GEN_TOKEN_GITHUB_COM",
		"gitlab.example.com": "GIT_PROTO_GEN_TOKEN_GITLAB_EXAMPLE_COM",
		"git.internal:8443":  "GIT_PROTO_GEN_TOKEN_GIT_INTERNAL_8443",
	}
	for host, want := range tests {
		if got := EnvVarForHost(host); got != want {
			t.Errorf("EnvVarForHost(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestEnvAuthResolverPrecedence(t *testing.T) {
	env := map[string]string{
		"GIT_PROTO_GEN_TOKEN_GITHUB_COM": "env-token",
	}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }

	t.Run("env var wins for that host", func(t *testing.T) {
		r := &EnvAuthResolver{legacyToken: "legacy", lookupEnv: lookup, sshAvailable: func() bool { return true }}
		cred, err := r.Resolve("github.com")
		if err != nil {
			t.Fatal(err)
		}
		if cred.Kind != KindToken || cred.Token() != "env-token" {
			t.Errorf("got kind=%v token=%q, want token/env-token", cred.Kind, cred.Token())
		}
	})

	t.Run("legacy token only applies to github.com", func(t *testing.T) {
		r := &EnvAuthResolver{legacyToken: "legacy", lookupEnv: lookup, sshAvailable: func() bool { return false }}
		cred, _ := r.Resolve("gitlab.example.com")
		if cred.Kind != KindAnonymous {
			t.Errorf("gitlab with only github legacy token: got %v, want anonymous", cred.Kind)
		}
	})

	t.Run("auth-config host token", func(t *testing.T) {
		r := &EnvAuthResolver{hostTokens: map[string]string{"gitlab.example.com": "cfg-token"}, lookupEnv: lookup, sshAvailable: func() bool { return false }}
		cred, _ := r.Resolve("gitlab.example.com")
		if cred.Kind != KindToken || cred.Token() != "cfg-token" {
			t.Errorf("got kind=%v token=%q, want token/cfg-token", cred.Kind, cred.Token())
		}
	})

	t.Run("ssh fallback", func(t *testing.T) {
		r := &EnvAuthResolver{lookupEnv: lookup, sshAvailable: func() bool { return true }}
		cred, _ := r.Resolve("bitbucket.org")
		if cred.Kind != KindSSH {
			t.Errorf("got %v, want ssh", cred.Kind)
		}
	})

	t.Run("anonymous when nothing available", func(t *testing.T) {
		r := &EnvAuthResolver{lookupEnv: lookup, sshAvailable: func() bool { return false }}
		cred, _ := r.Resolve("bitbucket.org")
		if cred.Kind != KindAnonymous {
			t.Errorf("got %v, want anonymous", cred.Kind)
		}
	})
}

func TestCredentialLogValueRedacts(t *testing.T) {
	c := &Credential{Kind: KindToken, token: "super-secret"}
	if got := c.LogValue().String(); got == "super-secret" || got != "token(redacted)" {
		t.Errorf("LogValue() = %q, must redact the token", got)
	}
}
