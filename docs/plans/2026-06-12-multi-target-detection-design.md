# Multi-Target Detection Design

Date: 2026-06-12
Status: Approved

## Problem

When a kubectl command targets multiple objects positionally, safekubectl only
detects and reports the first one. Example:

```
k delete secret cert-a cert-b cert-c cert-d
```

produces a warning showing only `secret/cert-a`. The remaining three secrets
are deleted without appearing in the warning or the audit log.

Root cause: in `internal/parser/parser.go`, the positional-argument loop
assigns the first non-flag arg to `Resource`, the second to `Name`, and
silently discards everything after. `KubectlCommand` has no way to represent
more than one target.

Related mis-parses fixed by the same change:

- `kubectl delete pod/a pod/b` — `pod/b` is currently treated as a *name*,
  producing `pod/a` only.
- `kubectl delete pods,services foo` — the comma-separated type spec is
  treated as one opaque resource string.

## Scope

Fix all three positional multi-target forms kubectl accepts:

1. One type, many names: `delete secret a b c d`
2. Slash-form targets: `delete pod/a pod/b secret/c`
3. Comma-separated types: `delete pods,services foo` (cross product of
   types x names, matching kubectl's resource builder; without names, one
   type-only target per type)

Out of scope: unifying the CLI warning path with the file-based (`-f`) path;
changes to danger-detection logic (it keys off operation/namespace/cluster,
which are command-level and already correct).

## Design

### 1. Parser (`internal/parser`)

New type, replacing the single `Resource`/`Name` fields on `KubectlCommand`:

```go
type Target struct {
    Resource string // "secret", "pod"
    Name     string // empty for type-only targets (e.g. delete pods -l app=x)
}

type KubectlCommand struct {
    Operation string
    Targets   []Target // replaces Resource + Name
    // ... other fields unchanged
}
```

The positional-arg loop collects all positional args (flag handling
unchanged, so flags interleaved between names keep working), then interprets
them:

- If the first positional arg contains `/`: every positional arg is parsed
  as `type/name`, one target each. A non-slash arg in this mode is recorded
  leniently as a type-only target; kubectl itself rejects the command.
- Otherwise: the first positional arg is the type spec, split on `,` into
  types. Every remaining positional arg is a name. Targets are the cross
  product of types x names. With no names, one type-only target per type.

The parser remains best-effort and never blocks execution; kubectl validates
the real command.

`GetResourceDisplay() string` becomes `GetResourceDisplays() []string`
(each entry `type/name`, or `type` for type-only targets; empty list yields
`["<unknown>"]` at display time).

### 2. Checker (`internal/checker`)

`CheckResult.Resource string` becomes `Resources []string`, populated from
`cmd.GetResourceDisplays()`. Danger logic is unchanged.

### 3. Prompt (`internal/prompt`)

`DisplayWarningTo` renders a tree list, always — including the single-target
case. Resource list sits between Cluster and Command:

```
⚠️   DANGEROUS OPERATION DETECTED
├── Operation: delete
├── Namespace: istio-system
├── Cluster:   main-jt-dgs-id-p-utility-01
├── Resources affected:
│   ├── secret/cert-a
│   └── secret/cert-b
└── Command:   kubectl delete secret cert-a cert-b
```

An empty target list renders a single `<unknown>` entry. The
`--all-namespaces` and node-scoped namespace lines behave exactly as today.

### 4. Audit (`internal/audit`)

`Log()` writes `resources=[secret/a,secret/b]` instead of
`resource=secret/a`, mirroring the format `LogResources` already uses for
file-based commands.

Compatibility note: this changes the audit line format. Any external tooling
parsing the old `resource=` key must switch to `resources=[...]`.

## Testing

- Parser table tests: the original failing command (one type, four names);
  slash-form multi-target; mixed slash-form types; comma-separated types
  without names; comma types x names cross product; flags interleaved
  between names (`delete secret a -n foo b`); `--` separator; type-only
  selector form (`delete pods -l app=x`).
- Checker tests updated for the `Resources` slice.
- Prompt output tests for the tree rendering (single and multiple targets,
  all-namespaces, node-scoped).
- Audit test for the `resources=[...]` entry format.
- Main integration test: end-to-end run with multiple names confirms all
  targets appear in warning and audit output.

## Error Handling

No new error paths. The parser is lenient by design; malformed target
combinations pass through to kubectl, which rejects them with its own error.
Dry-run commands and non-dangerous operations are unaffected.
