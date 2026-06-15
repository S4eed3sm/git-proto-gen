package source

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Spec is a parsed, vendor-agnostic description of one remote proto source. The
// concrete clone URL is built from Host, RepoPath and the resolved credential
// at fetch time, because the transport (HTTPS vs SSH) depends on auth.
type Spec struct {
	Raw      string // original CLI token, for diagnostics
	Scheme   string // "https" | "ssh" | "" (infer from auth)
	Host     string // e.g. "github.com", "gitlab.example.com"
	RepoPath string // e.g. "owner/repo" or "group/sub/project" (no scheme, no .git)
	Subdir   string // in-repo path to the .proto files; "" => repo root
	Ref      string // branch or tag; "" => default branch
	Legacy   bool   // true when parsed via the deprecated github.com positional form
}

// scpForm matches an scp-style git URL such as "git@github.com:owner/repo".
var scpForm = regexp.MustCompile(`^[^/@]+@[^/@:]+:`)

// ParseSpec parses a remote repo spec of the form
//
//	[scheme://]<host>/<repo-path>[.git]//<subdir>[@<ref>]
//
// The "//" delimiter separates the clonable repository from the in-repo proto
// subdirectory, which is unambiguous at any group-nesting depth. For backward
// compatibility a github.com spec without "//" is parsed with the legacy
// positional rule (host/owner/repo/subdir) and flagged via Spec.Legacy.
func ParseSpec(raw string) (*Spec, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("empty repo spec")
	}

	spec := &Spec{Raw: raw}

	body, ref := splitRef(trimmed)
	spec.Ref = ref

	scheme, rest, scp := splitScheme(body)
	spec.Scheme = scheme

	hasSubdirSep := false
	if i := strings.Index(rest, "//"); i >= 0 {
		spec.Subdir = strings.Trim(rest[i+2:], "/")
		rest = rest[:i]
		hasSubdirSep = true
	}
	rest = strings.TrimRight(rest, "/")

	if scp {
		host, repoPath, err := splitSCP(rest)
		if err != nil {
			return nil, err
		}
		spec.Host = host
		spec.RepoPath = normalizeRepoPath(repoPath)
		if spec.Host == "" || spec.RepoPath == "" {
			return nil, fmt.Errorf("invalid ssh repo spec %q", raw)
		}
		return spec, validateSpec(spec)
	}

	host, repoPath, ok := strings.Cut(rest, "/")
	if !ok {
		return nil, fmt.Errorf("invalid repo spec %q: expected <host>/<repo-path>//<subdir>", raw)
	}
	// Strip any "user@" userinfo from an explicit scheme URL (e.g. ssh://git@host/...).
	if i := strings.LastIndexByte(host, '@'); i >= 0 {
		host = host[i+1:]
	}
	spec.Host = host
	repoPath = strings.Trim(repoPath, "/")

	if hasSubdirSep {
		spec.RepoPath = normalizeRepoPath(repoPath)
		return spec, validateSpec(spec)
	}

	// No "//": only the legacy github.com positional form is accepted, since the
	// repo/subdir boundary cannot be inferred for arbitrary (nested-group) hosts.
	if spec.Host != "github.com" {
		return nil, fmt.Errorf(
			"repo spec %q for host %q must separate repo and proto subdir with '//', e.g. %s//proto@main",
			raw, spec.Host, rest)
	}
	segs := strings.SplitN(repoPath, "/", 3) // owner, repo, subdir
	if len(segs) < 2 {
		return nil, fmt.Errorf("invalid github repo spec %q: expected github.com/<owner>/<repo>", raw)
	}
	spec.Legacy = true
	spec.RepoPath = segs[0] + "/" + segs[1]
	if len(segs) == 3 {
		spec.Subdir = segs[2]
	}
	return spec, validateSpec(spec)
}

// RepoName returns the repository's base name (the last path segment).
func (s *Spec) RepoName() string {
	if i := strings.LastIndex(s.RepoPath, "/"); i >= 0 {
		return s.RepoPath[i+1:]
	}
	return s.RepoPath
}

// cloneURL builds the URL passed to `git clone`. SSH is used when the spec
// requested it explicitly or when the resolved credential is an SSH key;
// otherwise HTTPS is used (any token is supplied out-of-band, never in the URL).
func (s *Spec) cloneURL(useSSH bool) string {
	repo := s.RepoPath + ".git"
	if s.Scheme == "ssh" || (s.Scheme == "" && useSSH) {
		return fmt.Sprintf("git@%s:%s", s.Host, repo)
	}
	return fmt.Sprintf("https://%s/%s", s.Host, repo)
}

// splitRef separates a trailing "@<ref>". It ignores an scp-style userinfo "@"
// (one followed by "host:" before any "/"), so "git@host:owner/repo@main"
// yields ref "main".
func splitRef(s string) (rest, ref string) {
	at := strings.LastIndex(s, "@")
	if at < 0 {
		return s, ""
	}
	after := s[at+1:]
	colon := strings.IndexByte(after, ':')
	slash := strings.IndexByte(after, '/')
	if colon >= 0 && (slash < 0 || colon < slash) {
		// The last "@" is scp userinfo, not a ref separator.
		return s, ""
	}
	return s[:at], after
}

// splitScheme strips a leading "scheme://" or recognizes an scp-style URL.
func splitScheme(s string) (scheme, rest string, scp bool) {
	if sch, r, ok := strings.Cut(s, "://"); ok {
		return sch, r, false
	}
	if scpForm.MatchString(s) {
		return "ssh", s, true
	}
	return "", s, false
}

// splitSCP parses "user@host:path" into host and path.
func splitSCP(s string) (host, repoPath string, err error) {
	at := strings.IndexByte(s, '@')
	colon := strings.IndexByte(s, ':')
	if at < 0 || colon < at {
		return "", "", fmt.Errorf("invalid scp-form url %q", s)
	}
	return s[at+1 : colon], s[colon+1:], nil
}

func normalizeRepoPath(p string) string {
	return strings.TrimSuffix(strings.Trim(p, "/"), ".git")
}

// validateSpec rejects subdirs that escape the repository.
func validateSpec(s *Spec) error {
	if s.Subdir == "" {
		return nil
	}
	if strings.HasPrefix(s.Subdir, "/") {
		return fmt.Errorf("invalid subdir %q in spec %q: must be repo-relative", s.Subdir, s.Raw)
	}
	clean := path.Clean(s.Subdir)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("invalid subdir %q in spec %q: path traversal is not allowed", s.Subdir, s.Raw)
	}
	return nil
}
