# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Test Commands

```bash
# Build
go build -o safekubectl .

# Run all tests
go test ./... -v

# Run tests with coverage
go test ./... -cover

# Run a single package's tests
go test ./internal/parser -v

# Run a specific test
go test ./internal/parser -v -run TestParse
```

## Architecture

safekubectl is a kubectl wrapper that intercepts commands and warns/prompts before dangerous operations.

**Execution flow** (main.go):
1. `Runner.Run()` orchestrates the entire flow
2. Loads config via `config.Load()`
3. Parses kubectl args via `parser.Parse()`
4. Gets current cluster context via `kubectl config current-context`
5. Checks if command is dangerous via `checker.Check()`
6. If dangerous: displays warning, prompts for confirmation (or auto-proceeds in warn-only mode)
7. Logs to audit if enabled
8. Executes kubectl via `os/exec`

**Internal packages** (`internal/`):
- `config` - YAML config loading from `~/.safekubectl/config.yaml` or `SAFEKUBECTL_CONFIG` env var. Contains `Config` struct and helper methods like `IsDangerousOperation()`, `IsProtectedNamespace()`, `RequiresConfirmation()`
- `parser` - Parses kubectl args into `KubectlCommand` struct (operation, resource, name, namespace). Handles various flag formats (`-n`, `--namespace`, `--namespace=`)
- `checker` - Evaluates parsed commands against config to produce `CheckResult` with danger status and reasons
- `prompt` - Terminal output (colored warnings) and user confirmation prompts
- `audit` - Writes timestamped log entries to audit file when enabled

**Key types**:
- `config.Config` - Main configuration with mode, dangerous operations list, protected namespaces/clusters
- `parser.KubectlCommand` - Parsed kubectl command structure
- `checker.CheckResult` - Contains danger assessment (IsDangerous, RequiresConfirmation, Reasons)
