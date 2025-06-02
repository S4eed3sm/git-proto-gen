# git-proto-gen

**git-proto-gen** is a CLI tool that wraps the [Buf](https://buf.build) code generator, simplifying the process of generating client code from `.proto` files—both locally and from remote GitHub repositories (public or private).

---

## ✨ Features

- 🚀 Generate code using [Buf](https://buf.build) with a single command
- 📦 Fetch `.proto` files from:
  - Local directories
  - Public GitHub repositories
  - Private GitHub repositories (via `GitHub API Token` or `SSH-Key`)
- 🧬 Supports multiple languages: **Go** and **JavaScript**
- 🐳 Runs in Docker for consistent and dependency-free builds

---

## 🛠️ Prerequisites

- [Go](https://golang.org/) 1.16 or higher
- [Docker](https://www.docker.com/)

---

## 📦 Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/yourusername/git-proto-gen.git
   cd git-proto-gen
   ```

2. Build the binary:

   ```bash
   go build -o git-proto-gen
   ```

---

## 🚀 Usage

Run the tool with your desired configuration:

```bash
./git-proto-gen \
  --local proto \
  --private-repo github.com/S4eed3sm/private-test-proto/proto \
  --public-repo github.com/S4eed3sm/public-test-proto/proto/greeting.proto \
  --public-repo github.com/S4eed3sm/public-test-proto/proto/greeting_service.proto \
  --lang go \
  --lang js \
  --token $GITHUB_TOKEN
```

> 💡 For SSH access (instead of GitHub tokens), make sure your SSH agent is running and keys are loaded and remove --token argument.

---

## ⚙️ CLI Options

```
Usage:
  git-proto-gen [flags]

Flags:
      --buf-configs string     Path to optional buf config files (buf.yaml, buf.gen.go.yaml, buf.gen.js.yaml)
  -h, --help                   help for git-proto-gen
      --lang strings           Target language(s) for code generation: go, js (comma-separated or repeatable) (default [go,js])
      --local string           Path to local .proto files, e.g: './proto' (default "proto")
      --output string          Output directory for generated files (default "events")
      --private-repo strings   GitHub path(s) to private proto repos (repeatable, comma-separated), e.g: "github.com/S4eed3sm/private-test-proto/proto"
      --public-repo strings    GitHub path(s) to public proto repos (repeatable, comma-separated), e.g: "github.com/S4eed3sm/public-test-proto/proto"
```

---

## 🧬 How It Works

1. Creates a temporary workspace and merges local and remote `.proto` files.
2. Clones remote repositories using HTTPS (with token) or SSH.
3. Runs a Docker container using the `bufbuild/buf` image.
4. Uses `buf generate` with the appropriate templates.
5. Outputs generated code to the specified directory.

---

## 📄 License

Licensed under the [MIT License](LICENSE).
