# safekubectl

[![CI](https://github.com/zufardhiyaulhaq/safekubectl/actions/workflows/ci.yml/badge.svg)](https://github.com/zufardhiyaulhaq/safekubectl/actions/workflows/ci.yml)
[![Release](https://github.com/zufardhiyaulhaq/safekubectl/actions/workflows/release.yml/badge.svg)](https://github.com/zufardhiyaulhaq/safekubectl/actions/workflows/release.yml)

A kubectl wrapper that warns and prompts for confirmation before executing dangerous operations.

## Features

- Warns before dangerous operations (delete, apply, patch, edit, drain, exec, cordon, taint, rollout)
- Configurable confirmation modes (confirm or warn-only)
- Protected namespaces and clusters that always require confirmation
- Audit logging for dangerous operations
- Fully configurable via YAML config file

## Installation

### Download Binary

Download the latest release from the [GitHub Releases](https://github.com/zufardhiyaulhaq/safekubectl/releases) page.

```bash
# Linux (amd64)
curl -LO https://github.com/zufardhiyaulhaq/safekubectl/releases/latest/download/safekubectl-linux-amd64.tar.gz
tar -xzf safekubectl-linux-amd64.tar.gz
sudo mv safekubectl /usr/local/bin/safekubectl

# Linux (arm64)
curl -LO https://github.com/zufardhiyaulhaq/safekubectl/releases/latest/download/safekubectl-linux-arm64.tar.gz
tar -xzf safekubectl-linux-arm64.tar.gz
sudo mv safekubectl /usr/local/bin/safekubectl

# macOS (Intel)
curl -LO https://github.com/zufardhiyaulhaq/safekubectl/releases/latest/download/safekubectl-darwin-amd64.tar.gz
tar -xzf safekubectl-darwin-amd64.tar.gz
sudo mv safekubectl /usr/local/bin/safekubectl

# macOS (Apple Silicon)
curl -LO https://github.com/zufardhiyaulhaq/safekubectl/releases/latest/download/safekubectl-darwin-arm64.tar.gz
tar -xzf safekubectl-darwin-arm64.tar.gz
sudo mv safekubectl /usr/local/bin/safekubectl
```

### From Source

```bash
# Clone the repository
git clone https://github.com/zufardhiyaulhaq/safekubectl.git
cd safekubectl

# Build
go build -o safekubectl .

# Install to PATH (optional)
sudo mv safekubectl /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/zufardhiyaulhaq/safekubectl/cmd/safekubectl@latest
```

## Usage

Use `safekubectl` as a drop-in replacement for `kubectl`:

```bash
# Safe operations pass through without prompts
safekubectl get pods
safekubectl describe deployment nginx

# Dangerous operations show warning and require confirmation
safekubectl delete pod nginx -n production
```

### Example Output

```
⚠️  DANGEROUS OPERATION DETECTED
├── Operation: delete
├── Resource:  pod/nginx
├── Namespace: production
└── Cluster:   prod-us-east-1

Proceed? [y/N]:
```

## Configuration

Configuration file location: `~/.safekubectl/config.yaml`

You can override the config path using the `SAFEKUBECTL_CONFIG` environment variable:

```bash
export SAFEKUBECTL_CONFIG=/path/to/config.yaml
```

### Default Configuration

If no config file exists, safekubectl uses these defaults:

```yaml
# Confirmation mode: "confirm" (require y/N) or "warn-only" (display warning and proceed)
mode: confirm

# Operations considered dangerous
dangerousOperations:
  - delete
  - apply
  - patch
  - edit
  - update
  - rollout
  - drain
  - exec
  - cordon
  - taint

# Namespaces that always require confirmation regardless of mode
protectedNamespaces:
  - kube-system

# Clusters/contexts that always require confirmation regardless of mode
protectedClusters: []

# Audit logging configuration
audit:
  enabled: false
  path: ~/.safekubectl/audit.log
```

### Configuration Options

#### `mode`

| Value | Description |
|-------|-------------|
| `confirm` | Display warning and require `y/N` confirmation (default) |
| `warn-only` | Display warning and proceed automatically |

Note: Protected namespaces and clusters always require confirmation, even in `warn-only` mode.

#### `dangerousOperations`

List of kubectl operations that trigger warnings. Default includes:
- `delete` - Delete resources
- `apply` - Apply configuration changes
- `patch` - Patch resources
- `edit` - Edit resources in-place
- `update` - Update resources
- `rollout` - Rollout operations (restart, undo, etc.)
- `drain` - Drain nodes
- `exec` - Execute commands in containers
- `cordon` - Mark nodes as unschedulable
- `taint` - Add taints to nodes

#### `protectedNamespaces`

Namespaces that always require confirmation, even in `warn-only` mode:

```yaml
protectedNamespaces:
  - kube-system
  - production
  - prod
```

#### `protectedClusters`

Cluster contexts that always require confirmation:

```yaml
protectedClusters:
  - prod-us-east-1
  - prod-eu-west-1
```

#### `audit`

Enable audit logging to track dangerous operations:

```yaml
audit:
  enabled: true
  path: ~/.safekubectl/audit.log
```

Audit log format:
```
[2024-01-15T10:30:00+00:00] EXECUTED | operation=delete resource=pod/nginx namespace=production cluster=prod-us-east-1 confirmed=true command="delete pod nginx -n production"
[2024-01-15T10:31:00+00:00] DENIED | operation=delete resource=deployment/web namespace=production cluster=prod-us-east-1 confirmed=false command="delete deployment web -n production"
```

## Example Configurations

### Production-Safe Configuration

```yaml
mode: confirm

dangerousOperations:
  - delete
  - apply
  - patch
  - edit
  - update
  - rollout
  - drain
  - exec
  - cordon
  - taint

protectedNamespaces:
  - kube-system
  - production
  - prod
  - default

protectedClusters:
  - prod-us-east-1
  - prod-us-west-2
  - prod-eu-west-1

audit:
  enabled: true
  path: ~/.safekubectl/audit.log
```

### Development Configuration (Less Strict)

```yaml
mode: warn-only

dangerousOperations:
  - delete
  - drain

protectedNamespaces:
  - kube-system

protectedClusters: []

audit:
  enabled: false
```

## Shell Alias (Optional)

To use `safekubectl` as your default kubectl, add an alias:

```bash
# Add to ~/.bashrc or ~/.zshrc
alias kubectl='safekubectl'
```

## Development

### Run Tests

```bash
go test ./... -v
```

### Run Tests with Coverage

```bash
go test ./... -cover
```

### Build

```bash
go build -o safekubectl .
```

## License

MIT License
