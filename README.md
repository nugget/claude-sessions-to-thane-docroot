# claude-sessions-to-thane-docroot

Export a Claude Code project's session transcripts into a hierarchy of markdown
documents that form a [`thane-ai-agent`](https://github.com/nugget/thane-ai-agent)
**document root**.

The goal is to turn raw session logs into *primary source material* for tracing
the ebbs and flows of a project's development — readable by a human and
indexable by Thane. One markdown document per session, grouped by the git branch
the work happened on.

> The binary is `transcript-exporter`. The repo is named for what it does.

## What it reads and writes

- **Reads** the JSONL transcripts Claude Code stores under
  `~/.claude/projects/<encoded-project-path>/` (and the worktree-sibling
  directories alongside them).
- **Writes** a tree of markdown into a target directory you nominate as a Thane
  document root.

```
<target>/
  README.md                              # root overview (document_kind: transcript_root_overview)
  main/
    README.md                            # per-branch index, sessions in date order
    2026-05-26 thane-primary-development.md
  fix/issue-788-close-verifier/
    README.md
    2026-04-29 close-the-verifier-bypass.md
  _unbranched/                           # sessions with no recorded git branch
    ...
```

Branch names become directories (slashes are preserved as real subdirectories).
Each session document is named `YYYY-MM-DD <title-slug>.md`.

## Quick start

```sh
# Build it
just build            # → ./transcript-exporter
# …or run straight from source
just run --project thane-ai-agent --target ~/Thane/transcripts --root-name transcripts --dry-run
```

```sh
# Real export (idempotent; safe to re-run)
transcript-exporter \
  --project   thane-ai-agent \
  --target    ~/Thane/transcripts \
  --root-name transcripts
```

`--project` accepts any of: a bare project name (matched against the encoded
project dirs), the project's repo path, or the encoded `~/.claude/projects`
directory name.

### Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--project` | *(required)* | Which Claude project to export. |
| `--target` | *(required)* | Output document-root directory to sync. |
| `--root-name` | basename of `--target` | Thane root name; fills frontmatter `managed_root` and `<root>:` refs. |
| `--worktrees` | `true` | Merge worktree-sibling project dirs into the same root. |
| `--dry-run` | `false` | Report what would change; write nothing. |
| `--thinking` | `true` | Include assistant thinking blocks in the narrative. |
| `--max-thinking-chars` | `1600` | Cap per thinking block. |
| `--max-result-lines` | `30` | Cap tool-result text (lines). |
| `--max-result-bytes` | `4000` | Cap tool-result text (bytes). |
| `--timezone` | `Local` | Timezone for document dates: `Local`, `UTC`, or an IANA name. |
| `--verbose` | `false` | Debug logging. |

## How a session is rendered

Each session document is a *narrative*, not a JSON dump:

- a lead paragraph (the opening request) — **this is what Thane indexes as the
  document summary**;
- an **Overview** (time span, branch(es), working dir, activity counts, tools
  used, linked PRs, Claude Code version);
- a **Conversation** section: human prompts (block-quoted), assistant prose,
  thinking (collapsible), tool calls humanized to one line with their output
  folded into `<details>`, and PR links.

## Idempotency / sync model

Output is a deterministic function of the transcripts, so re-running is a
push-sync of the target directory:

- a file is written only when its bytes actually change;
- documents this tool owns (identified by the `generated_by:
  "session-transcript-exporter"` frontmatter marker) that are no longer produced
  are **deleted**, and emptied directories are pruned;
- files *without* that marker — your own notes, `.git`, signing material — are
  never touched.

The tool performs **no git operations**. If you want Thane's
`verify_signatures: required` policy, you commit and sign the target directory
yourself (see below).

> ⚠️ **Privacy.** Transcripts contain everything typed and read during a session
> — file contents, environment values, occasionally secrets. Review before
> publishing a transcripts root or pushing it to a remote. Redaction is not yet
> implemented.

## Wiring the root into Thane

Register the target directory as a document root in Thane's config. The
`--root-name` you exported with should match the root key:

```yaml
paths:
  transcripts: ~/Thane/transcripts

doc_roots:
  transcripts:
    authoring: read_only        # this corpus is generated, not hand-edited
    git:
      enabled: true
      sign_commits: true        # if YOU commit the target dir with a trusted key
      verify_signatures: warn   # or: required
      signing_key: ~/.ssh/id_ed25519
```

Frontmatter is tailored to Thane's reader on purpose. Its document parser is
**line-based, not full YAML** — flat keys, scalar / inline-array / block-list
values only, all list values deduped and sorted, keys lowercased. The emitter
here only ever produces that subset. The authoritative contract lives in the
`thane-ai-agent` repo at `docs/understanding/document-roots.md` and
`internal/state/documents/parser.go`; re-check it before changing the emitted
frontmatter.

Emitted frontmatter fields: `title`, `description`, `tags`, `generated_by`,
`generated_at`, `document_kind`, `refresh_strategy`, `source_refs`
(`conversation:<id>`, `branch:<b>`, `pr:<n>`, `cwd:<path>`, `project:<name>`),
`managed_root`, `created`, `updated`.

## How it works (architecture)

The pipeline is five small packages, each a stage:

```
claudeproj  →  transcript  →  export (place)  →  render  →  docroot (sync)
 locate         parse           group/order      markdown    reconcile target
```

| Package | Responsibility |
| --- | --- |
| `internal/claudeproj` | Resolve a `--project` to its `~/.claude/projects` dir(s); enumerate session `.jsonl` files (worktree siblings merged, subagent sidecars skipped). |
| `internal/transcript` | Tolerant JSONL → `Session` model: prompts, assistant text/thinking, tool calls paired with results, branches, PRs, titles. |
| `internal/render` | `Session` → markdown. Frontmatter emitter (Thane's subset), per-session narrative, branch index, root overview. Slugging, branch→dir mapping, filenames. |
| `internal/export` | Orchestration: parse all sessions, group by home branch, order chronologically, assign collision-free filenames, render, hand the file set to the sync layer. |
| `internal/docroot` | Idempotent reconcile of the desired file set against the target directory (write-on-change, marker-scoped delete, prune). |
| `cmd/transcript-exporter` | Flag parsing and wiring. |

### Transcript-format quirks worth knowing

These are the non-obvious things the parser handles — useful when something
looks wrong:

- **Out-of-order tool I/O.** When tools run in parallel, Claude Code can log a
  `tool_result` *before* its `tool_use`. Results that arrive early are buffered
  (`pending` map in `parse.go`) and attached when the call appears.
- **PR spam.** `pr-link` entries repeat on every CI poll; PRs are deduped per
  session.
- **Titles.** Come from `custom-title` / `ai-title` entries (not a `summary`
  entry). Resolution order: custom → AI → first prompt → session id.
- **Subagents.** Live in a `<sessionId>/subagents/` sidecar. Their internal
  steps are *not* rendered today; the `Task` tool's result already carries the
  subagent's returned report, which is what the narrative shows.
- **Injected envelopes.** `<system-reminder>`, `<local-command-stdout>`, etc.
  are stripped from user text; slash-command invocations are surfaced as
  `Ran command /foo`.

## Extending it

Common changes and where they live:

- **Make a tool render better** (e.g. a new MCP tool, or richer Bash output):
  `humanizeTool` in `internal/transcript/tools.go`. Return a one-line summary,
  optional detail, and (for dispatchers) a `SubagentRef`.
- **Change the directory layout or filenames:** `placeSessions` /
  `assignFilenames` in `internal/export/export.go`, plus `BranchDir` / `Slug` in
  `internal/render/slug.go`.
- **Add or change a frontmatter field:** `Frontmatter` + `Render` in
  `internal/render/frontmatter.go`, and the builders in `render/session.go` /
  `render/index.go`. Keep to Thane's parseable subset (flat keys; scalars or
  block lists).
- **Render subagent internals:** the sidecars are already enumerated by
  `claudeproj`; parse `<sessionId>/subagents/agent-*.jsonl` and splice into the
  `Task` tool-call event.

**Invariant to preserve: determinism.** Output must be a pure function of the
transcripts so the sync stays a clean no-op. Never put wall-clock time in a
document (`generated_at` is the session's *end* time, not "now"). If you add a
field, derive it from the data.

## Development

```sh
just            # list recipes
just test       # go test -race ./...
just lint       # golangci-lint (v2 config in .golangci.yml)
just ci         # full gate: fmt-check, vet, mod-tidy-check, lint, test
```

Standard library only — no third-party dependencies. CI runs `just ci` on push
and PR (`.github/workflows/ci.yml`).

## License

Apache 2.0. See [LICENSE](LICENSE).
