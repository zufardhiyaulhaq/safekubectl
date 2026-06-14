# JSON Audit Output Design

Date: 2026-06-14
Status: Approved

## Problem

The audit logger writes only a plain-text `key=value` line per entry. Machine
consumers (log shippers, SIEM, `jq` pipelines) have to parse an ad-hoc text
format. We want an opt-in JSON output mode while keeping text as the default.

A secondary issue: `audit.Log` (CLI commands) and `audit.LogResources`
(file-based `-f` commands) duplicate the same boilerplate — enabled check,
directory creation, append-open, timestamp, status — and each hand-formats its
own line. Adding a second output format to both would multiply that
duplication.

## Scope

- Add a config-selected output format: `text` (default) or `json`.
- JSON output is JSON Lines (one JSON object per line, newline-terminated).
- Refactor the two log methods onto one shared entry type and writer so the
  format is chosen in exactly one place.

Out of scope: changing what gets logged (fields, danger logic), structured
per-resource objects in JSON (resources stay as the joined strings already
produced), CLI flags or env vars for format selection.

## Decisions

- **Format selection:** a YAML config field, consistent with `mode`,
  `audit.enabled`, `audit.path`. Not a CLI flag (safekubectl forwards unknown
  args to kubectl; a new flag risks collisions) and not an env var (splits
  config across mechanisms).
- **Resource representation in JSON:** keep the joined strings exactly as the
  text format produces them — `"secret/cert-a"` for the CLI path,
  `"Deployment/nginx@istio-system"` for the file path. No changes to
  checker/parser.
- **Unified schema for both paths:** every JSON line carries the same keys.
  The file-based path leaves `namespace` empty (its namespace is per-resource,
  baked into each resource string).
- **Unify text too:** both paths render through one `Entry` and one text
  layout. This changes the file-based path's text line (it gains an empty
  `namespace=` field and reorders `cluster` after `namespace`). Accepted: the
  text audit format already changed recently (`resource=` → `resources=[...]`),
  and a single layout removes the per-path branch entirely.
- **JSON Lines, not a JSON array:** the log is append-only; an array would
  require rewriting the file. JSONL streams and greps cleanly (`jq -c`).

## Design

### 1. Config (`internal/config`)

Add one field to `AuditConfig`:

```go
type AuditConfig struct {
    Enabled bool   `yaml:"enabled"`
    Path    string `yaml:"path"`
    Format  string `yaml:"format"` // "text" (default) or "json"
}
```

An empty or unrecognized value resolves to `text` at write time, so existing
configs and existing log files are unaffected. `config.example.yaml` gains a
documented `format: text` line under `audit`.

### 2. Unified entry and shared writer (`internal/audit`)

One entry type, JSON-tagged, is the single source of truth for an audit
record:

```go
type Entry struct {
    Timestamp string   `json:"timestamp"`
    Status    string   `json:"status"`     // EXECUTED | DENIED
    Operation string   `json:"operation"`
    Resources []string `json:"resources"`
    Namespace string   `json:"namespace"`  // empty for file-based commands
    Cluster   string   `json:"cluster"`
    Confirmed bool     `json:"confirmed"`
    Command   string   `json:"command"`
}
```

`Log` and `LogResources` keep only their distinct responsibility — building the
resource string list (`secret/cert-a` vs `Deployment/nginx@istio-system`) and
setting `Namespace` — then construct an `Entry` and hand it to a shared
private method:

```go
func (l *Logger) writeEntry(e Entry) error
```

`writeEntry` owns everything that was duplicated:
1. Return early if `!l.config.Audit.Enabled`.
2. `os.MkdirAll(filepath.Dir(path), 0755)`.
3. Open the file with `O_APPEND|O_CREATE|O_WRONLY`, `0644`.
4. Render the line via `formatText` or `formatJSON` based on
   `l.config.Audit.Format`.
5. Write the line (newline-terminated) and return any error, wrapped as today.

Timestamp (`time.Now().Format(time.RFC3339)`) and status (`EXECUTED`/`DENIED`
from the `executed` bool) are computed in `Log`/`LogResources` when building
the `Entry`, so `writeEntry` stays purely about persistence and formatting.

### 3. Formatters (`internal/audit`)

```go
func formatText(e Entry) string  // unified key=value line, both paths
func formatJSON(e Entry) (string, error)
```

- **Text:** a single layout for both paths:

  ```
  [<timestamp>] <status> | operation=<op> resources=[<r1,r2>] namespace=<ns> cluster=<cluster> confirmed=<bool> command="<args>"
  ```

  `resources` joins the entry's `Resources` with `,`. For the file-based path
  `namespace` renders empty (`namespace= `).

- **JSON:** `json.Marshal(e)` plus a trailing newline — one object per line.

### Data flow

```
Runner ──> Log(result, args, confirmed, executed)          ─┐
       └─> LogResources(result, args, confirmed, executed) ─┤
                                                             ├─> build Entry ─> writeEntry(Entry)
                                                             │                     ├─ enabled? dir? open
                                                             │                     └─ formatText | formatJSON
                                                             ┘
```

## Example output

CLI: `kubectl delete secret cert-a cert-b -n istio-system`, executed/confirmed,
cluster `prod-cluster`.

File: `kubectl apply -f deploy.yaml` with `Deployment/nginx` in `istio-system`,
executed/confirmed, cluster `prod-cluster`.

**text (default):**

```
[2026-06-14T10:38:14Z] EXECUTED | operation=delete resources=[secret/cert-a,secret/cert-b] namespace=istio-system cluster=prod-cluster confirmed=true command="delete secret cert-a cert-b -n istio-system"
[2026-06-14T10:38:14Z] EXECUTED | operation=apply resources=[Deployment/nginx@istio-system] namespace= cluster=prod-cluster confirmed=true command="apply -f deploy.yaml"
```

**json:**

```json
{"timestamp":"2026-06-14T10:38:14Z","status":"EXECUTED","operation":"delete","resources":["secret/cert-a","secret/cert-b"],"namespace":"istio-system","cluster":"prod-cluster","confirmed":true,"command":"delete secret cert-a cert-b -n istio-system"}
{"timestamp":"2026-06-14T10:38:14Z","status":"EXECUTED","operation":"apply","resources":["Deployment/nginx@istio-system"],"namespace":"","cluster":"prod-cluster","confirmed":true,"command":"apply -f deploy.yaml"}
```

## Error handling

No new failure modes. `writeEntry` wraps mkdir/open/write errors exactly as the
current methods do. `formatJSON` returns the `json.Marshal` error (wrapped);
with the all-scalar `Entry` schema marshalling cannot realistically fail, but
the error is threaded rather than swallowed. An unrecognized `format` value is
not an error — it resolves to `text`.

## Backwards compatibility

- Default `format` is `text`; omitting the field keeps current behavior for the
  CLI path byte-for-byte.
- The file-based path's **text** line changes: it gains an empty `namespace=`
  field and `cluster` moves after `namespace`. Any parser of the old
  file-based text format must adjust. This is the deliberate cost of a single
  unified layout.

## Testing

- **config:** `format: json` parses; default (omitted) is `text`; unknown value
  is preserved as-is in the struct and resolves to text at write time.
- **audit text (CLI):** output is byte-for-byte unchanged from today.
- **audit text (file-based):** matches the new unified layout (empty
  `namespace=`, reordered `cluster`).
- **audit json:** each line is a single valid JSON object with all eight keys;
  multi-resource entry serializes `resources` as a JSON array; file-based entry
  has `"namespace":""`; `status` is `EXECUTED` vs `DENIED` per the `executed`
  flag; `confirmed` reflects the bool.
- **disabled:** with `Audit.Enabled=false`, nothing is written in either format.
- **shared writer:** both `Log` and `LogResources` honor the configured format
  (parameterize a test over `text`/`json`).
