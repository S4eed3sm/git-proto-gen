# git-proto-gen

**GitProtoGen** is a CLI tool that wraps the [Buf](https://buf.build) code generator, making it easy to generate client code from `.proto` files—both locally and from remote Git repositories (public or private).

---

## ✨ Features

- 🚀 Generate code using [Buf](https://buf.build) with a single command.
- 📦 Supports fetching `.proto` files from:
  - Local directories
  - Public GitHub repositories
  - Private GitHub repositories
- 🧬 Supports multiple languages: **Go** and **JavaScript**
- 🐳 Uses Docker to ensure consistency and avoid local dependencies

---

## 🛠️ Prerequisites

- [Go](https://golang.org/) 1.16+
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
./git-proto-gen  --local proto --private-repo github.com/S4eed3sm/private-test-proto/proto --public-repo github.com/S4eed3sm/public-test-proto/proto/greeting.proto  --public-repo github.com/S4eed3sm/public-test-proto/proto/greeting_service.proto   --lang go --lang js --token $GITHUB_TOKEN
```

---

## ⚙️ CLI Options

```
Usage:
  git-proto-gen [flags]

Flags:
  -h, --help                   Show help message
      --lang strings           Target language(s) for code generation: go, js (comma-separated or repeatable)
      --local string           Path to local .proto files (default: "proto")
      --output string          Output directory for generated files (default: "events")
      --private-repo strings   Private GitHub repo paths (repeatable), e.g. "github.com/user/private-repo/proto"
      --public-repo strings    Public GitHub repo paths (repeatable), e.g. "github.com/user/public-repo/proto"
      --token string           GitHub token (required for private repos)
```

---

## 🧬 How It Works

1. Creates a temporary workspace.
2. Clones remote `.proto` files into the workspace.
3. Starts a Docker container with the `bufbuild/buf` image.
4. Runs `buf generate` using language-specific templates.
5. Copies the generated code into your specified output directory.

---

## 📄 License

This project is licensed under the **MIT License**. See the [LICENSE](LICENSE) file for details.
