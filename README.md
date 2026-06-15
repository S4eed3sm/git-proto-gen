# git-proto-gen

**git-proto-gen** is a CLI tool that wraps the [Buf](https://buf.build) code generator, simplifying the process of generating client code from `.proto` files — from local directories and from remote git repositories on **any git vendor**: GitHub, self-hosted GitLab, Bitbucket, Gitea, GitHub Enterprise, and others.

---

## ✨ Features

- 🚀 Generate code using [Buf](https://buf.build) with a single command
- 📦 Fetch `.proto` files from:
  - Local directories
  - Any git host over HTTPS (with a token) or SSH — vendor-agnostic, via a single shallow `git clone`
- 🔐 Per-host credentials via environment variables, an auth-config file, the legacy `--token`, or SSH keys — tokens are never passed on the command line
- 🧬 Supports multiple languages: **Go** and **JavaScript**
- 🐳 Runs in Docker for consistent builds, or directly with a host `buf` binary

---

## 🛠️ Prerequisites

- [Go](https://golang.org/) 1.23 or higher (to build)
- [`git`](https://git-scm.com/) — required for fetching remote repositories
- [Docker](https://www.docker.com/) — required unless you use `--runner host` with a local [`buf`](https://buf.build) install

---

## 📦 Installation

```bash
git clone https://github.com/S4eed3sm/git-proto-gen.git
cd git-proto-gen
make build-darwin-arm64   # or: go build -o git-proto-gen ./cmd/git-proto-gen
```

`make all` cross-compiles for linux/darwin/windows on amd64/arm64. The version is derived from `git describe` and embedded into the binary (`git-proto-gen --version`).

---

## 🚀 Usage

```bash
./git-proto-gen \
  --local proto \
  --repo "github.com/S4eed3sm/public-test-proto//proto" \
  --repo "gitlab.example.com/group/subgroup/project//proto@main" \
  --lang go \
  --lang js \
  --output events
```

### Repo spec format

```text
[scheme://]<host>/<repo-path>[.git]//<subdir>[@<ref>]
```

The `//` separates the clonable **repository** from the in-repo **proto subdirectory**. This is unambiguous even for self-hosted GitLab with arbitrarily nested groups. `@<ref>` is an optional branch or tag.

| Spec | Repository | Subdir | Ref |
|------|------------|--------|-----|
| `github.com/o/repo//proto@dev` | `https://github.com/o/repo.git` | `proto` | `dev` |
| `gitlab.example.com/group/sub/proj//proto/dir@main` | `https://gitlab.example.com/group/sub/proj.git` | `proto/dir` | `main` |
| `ssh://git@gitlab.example.com/g/s/p.git//proto@v1` | `ssh://git@gitlab.example.com/g/s/p.git` | `proto` | `v1` |
| `git@github.com:o/repo.git//proto@main` | `git@github.com:o/repo.git` | `proto` | `main` |

> **Backward compatibility:** the old positional form `github.com/owner/repo/proto@branch` (no `//`) still works for `github.com` and prints a deprecation warning. For any other host the `//` separator is required.

### Authentication

Credentials are resolved **per host**, in this order:

1. `GIT_PROTO_GEN_TOKEN_<HOST>` env var (host upper-cased, non-alphanumerics → `_`), e.g. `GIT_PROTO_GEN_TOKEN_GITLAB_EXAMPLE_COM`.
2. `--token` — legacy, applies to `github.com` only.
3. `--auth-config <file>` — a YAML map of host → credential:
   ```yaml
   hosts:
     gitlab.example.com: { tokenEnv: GITLAB_PAT }   # read the secret from $GITLAB_PAT
     bitbucket.org:      { token: app-password }    # inline (discouraged)
   ```
4. An SSH key in `~/.ssh` (`id_rsa`, `id_ed25519`, `id_ecdsa`) → clone over SSH.
5. Anonymous HTTPS (public repos).

Tokens are passed to `git` through a credential helper via the environment, never as a command-line argument, so they do not appear in `ps`.

### CLI options

```text
--local string             path to local .proto files
--repo strings             remote proto repo(s), form host/owner/repo//subdir@ref (repeatable)
--output string            destination directory for generated files (default "events")
--lang strings             target language(s): go, js (default [go,js])
--token string             legacy github.com token (prefer env var / --auth-config)
--auth-config string       path to a per-host credential YAML file
--buf-configs string       directory with override buf config files
--namespace-imports        rewrite proto imports to namespace them by repo name
--runner string            buf runner: auto, docker, or host (default "auto")
--image string             buf Docker image (default "bufbuild/buf")
--image-tag string         buf Docker image tag (default "1.54.0")
--memory-gib int           container memory limit in GiB (default 2)
--version                  print version
```

`--private-repo` / `--public-repo` are kept as deprecated aliases for `--repo`.

### Merging multiple repos (`--namespace-imports`)

When you merge `.proto` files from several repositories, top-level paths can collide. `--namespace-imports` (off by default, since it rewrites proto source) prefixes each repo's module-root-relative imports with the repository name. Imports that resolve outside the repo (e.g. `google/protobuf/*`) are left untouched.

---

## 🧬 How It Works

1. Renders the embedded buf configuration (or your `--buf-configs` overrides) into a temporary workspace.
2. Fetches every source into the workspace: local dirs are copied; remote repos are shallow/sparse-cloned over HTTPS or SSH.
3. Runs `buf generate` — once per language, reusing a single Docker container (or a host `buf` binary).
4. Copies the generated code to `--output`.
5. Cleans up all temporary directories and the container, including on Ctrl-C.

---

## 📄 License

Licensed under the [MIT License](LICENSE).
