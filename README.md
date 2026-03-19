# Go Tool Base (GTB)

A comprehensive Go framework for building mission-critical CLI tools. GTB provides a standardized foundation with dependency injection, service lifecycle management, AI-powered automation, and deep GitHub and GitLab integration.

> [!IMPORTANT]
> **Full Documentation**: For detailed guides, component deep-dives, and API references, please visit our documentation site:
> **[https://gtb.phpboyscout.uk](https://gtb.phpboyscout.uk)**

## 📦 CLI Installation

To install the `gtb` automation CLI, use the recommended installation script for your platform:

**macOS/Linux (bash/zsh):**
```bash
curl -sSL -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "Accept: application/vnd.github.v3.raw" "https://github.com/phpboyscout/gtb/raw/main/install.sh" | bash
```

**Windows (PowerShell):**
```powershell
$env:GITHUB_TOKEN = "your_token_here"
irm "https://github.com/phpboyscout/gtb/raw/main/install.ps1" -Headers @{Authorization = "Bearer $env:GITHUB_TOKEN"} | iex
```

> [!NOTE]
> For developers building from source, you can still use `go install github.com/phpboyscout/gtb@latest`. However, this method will not include pre-built documentation assets, and the `docs` command will operate in a limited "source-build" mode.

## 🚀 Key Features

- **Scaffold**: Generate production-ready CLI tools in seconds using the built-in generator.
- **AI-Powered**: Integrated support for Claude, Gemini, and OpenAI to power autonomous repair and documentation Q&A.
- **Robust Configuration**: Flexible loading from files, environment variables, and embedded assets with hot-reloading support.
- **🔄 Lifecycle Management**: Unified control for starting, stopping, and coordinating concurrent background services.
- **🏢 Enterprise VCS**: Deep integration with GitHub Enterprise and GitLab (including nested group paths) for repository operations, PR management, and release assets.
- **🩹 Error Handling**: Structured error management with stack traces, severity levels, and integrated help context.
- **🛡️ Multi-Factor Auth**: Built-in support for GitHub and GitLab authentication and SSH key management.
- **📦 Auto Updates**: Seamless version checking and self-update capabilities directly from GitHub and GitLab releases.

## 🏗️ Core Architecture

The framework is built around a centralized **Props** container that provides type-safe access to all system dependencies:

| Component | Responsibility |
| :--- | :--- |
| **[pkg/props](docs/components/props.md)** | Central dependency injection container for logger, config, and assets. |
| **[pkg/config](docs/components/config.md)** | Viper-powered configuration with observer patterns and testing mocks. |
| **[pkg/chat](docs/components/chat.md)** | Unified multi-provider AI client (Claude, OpenAI, Gemini, Claude Local). |
| **[pkg/controls](docs/components/controls.md)** | Service lifecycle management and message-based coordination. |
| **[pkg/setup](docs/components/setup.md)** | Bootstrap logic: auth, key management, and pluggable self-updating. |
| **[pkg/vcs](docs/components/version-control.md)** | Pluggable GitHub/GitLab API and Git operations abstraction. |
| **[pkg/errorhandling](docs/components/error-handling.md)** | Structured errors with stack traces and log integration. |

## 🛠️ Built-in Commands

Every tool built on GTB inherits these essential capabilities:

- **`init`**: Bootstraps local environments, configures GitHub/GitLab auth, and manages SSH keys.
- **`version`**: Reports the current version and checks for available updates.
- **`update`**: Downloads and installs the latest release binary from GitHub or GitLab.
- **`mcp`**: Exposes CLI commands as Model Context Protocol (MCP) tools for use in IDEs.
- **`docs`**: Interactive terminal browser for documentation with built-in AI Q&A.

Commands can be selectively enabled or disabled at bootstrap time via feature flags — see [Feature Flags](#-feature-flags) below.

## 🤖 AI Providers

GTB supports multiple AI providers via a unified `pkg/chat` interface:

| Provider | Constant | Notes |
| :--- | :--- | :--- |
| **Anthropic Claude** | `ProviderClaude` | Requires `ANTHROPIC_API_KEY` |
| **Claude Local** | `ProviderClaudeLocal` | Uses a locally installed `claude` CLI binary |
| **OpenAI** | `ProviderOpenAI` | Requires `OPENAI_API_KEY` |
| **OpenAI-Compatible** | `ProviderOpenAICompatible` | Any OpenAI-compatible endpoint |
| **Google Gemini** | `ProviderGemini` | Requires `GEMINI_API_KEY` |

Set the active provider with the `AI_PROVIDER` environment variable or in your tool's configuration.

## 🏁 Quick Start

The fastest way to create a new GTB-based tool is with the scaffold command:

```bash
gtb generate project
```

This launches an interactive wizard to configure your project. For automation:

```bash
gtb generate project --name mytool --repo myorg/mytool --description "My CLI tool" --path ./mytool
```

For a GitLab-hosted project with nested groups:

```bash
gtb generate project --name mytool --repo myorg/mygroup/mytool --git-backend gitlab --host gitlab.mycompany.com --path ./mytool
```

### Generated Project Structure

The scaffold produces a fully wired project. The key entry points are:

**`cmd/mytool/main.go`** — entry point, reads version from `internal/version`:
```go
func main() {
    rootCmd, p := root.NewCmdRoot(version.Get())
    gtbRoot.Execute(rootCmd, p)
}
```

**`pkg/cmd/root/cmd.go`** — wires the Props container and root command:
```go
//go:embed assets/*
var assets embed.FS

func NewCmdRoot(v pkgversion.Info) (*cobra.Command, *props.Props) {
    logger := log.NewWithOptions(os.Stderr, log.Options{ReportTimestamp: true})

    p := &props.Props{
        Tool: props.Tool{
            Name:        "mytool",
            Description: "My CLI tool",
            ReleaseSource: props.ReleaseSource{
                Type:  "github",
                Host:  "github.com",
                Owner: "myorg",
                Repo:  "mytool",
            },
        },
        Logger:  logger,
        FS:      afero.NewOsFs(),
        Version: v,
        Assets:  props.NewAssets(props.AssetMap{"root": &assets}),
    }
    p.ErrorHandler = errorhandling.New(logger, p.Tool.Help)

    return gtbRoot.NewCmdRoot(p), p
}
```

**`internal/version/version.go`** — populated from GoReleaser ldflags at release, or from `runtime/debug` VCS info in development:
```go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```

## 🏳️ Feature Flags

Commands can be selectively disabled or opt-in features enabled via the `Tool` configuration:

| Feature | Default | Description |
| :--- | :--- | :--- |
| `update` | **enabled** | Self-update capability |
| `init` | **enabled** | Environment bootstrap command |
| `mcp` | **enabled** | Model Context Protocol server |
| `docs` | **enabled** | Documentation browser |
| `ai` | disabled | AI-powered features (opt-in) |

```go
props.Tool{
    // ...
    Disable: []props.FeatureCmd{props.UpdateCmd},     // disable self-update
    Enable:  []props.FeatureCmd{props.AiCmd},         // opt-in to AI features
}
```

## 📂 Project Layout

Standard layout for GTB projects:

```
.
├── cmd/
│   └── mytool/
│       └── main.go              # Entry point
├── pkg/
│   └── cmd/
│       └── root/
│           ├── cmd.go           # Root command and Props setup
│           └── assets/
│               └── init/
│                   └── config.yaml  # Default configuration
├── internal/
│   └── version/
│       └── version.go           # Version info (ldflags + runtime/debug)
├── go.mod
└── README.md
```
