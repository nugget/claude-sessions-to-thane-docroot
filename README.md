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
- **Writes** a tree of markdown into **one shared Thane document root**, with
  every project under its own `<agent>/<project>/` subtree. A single root holds
  many projects — registering a separate Thane root per project is an
  operational headache, so don't.

```
~/Thane/agentic-coding/                        # the document root (one, shared)
  claude/
    thane-ai-agent/                            # one project's subtree
      README.md                                # project overview (document_kind: transcript_root_overview)
      2026-05/
        README.md                              # month index, sessions in time order
        2026-05-26 1431 thane-primary-development.md
      2026-04/
        2026-04-29 0915 close-the-verifier-bypass.md
  codex/
    some-other-project/                        # a sibling project, exported independently
```

The **session** is the unit of continuity, so within a project subtree sessions
are organized **chronologically** by calendar month (`YYYY-MM/`), one document
per session named `YYYY-MM-DD HHMM <title-slug>.md`. A single session usually
spans several branches — sometimes interleaved — so branch is *not* the folder:
each session records every branch it touched in frontmatter and the Overview,
and marks inline where the working branch switches mid-conversation. Each project
export is scoped to its own subtree, so re-running one project never disturbs
another.

## Quick start

```sh
# Build it
just build            # → ./transcript-exporter
# …or preview a run straight from source
just run --project thane-ai-agent --root ~/Thane/agentic-coding --subpath claude/thane-ai-agent --dry-run
```

```sh
# Real export (idempotent; safe to re-run)
transcript-exporter \
  --project thane-ai-agent \
  --root    ~/Thane/agentic-coding \
  --subpath claude/thane-ai-agent
```

`--root` is the shared document root; `--subpath` is where this project lands
within it. The root name stamped into every document's `managed_root` derives
from the root's basename (`agentic-coding` here), so it stays identical across
every project exported into the same root — nothing to remember, nothing to
drift.

`--project` accepts any of: a bare project name (matched against the encoded
project dirs), the project's repo path, or the encoded `~/.claude/projects`
directory name.

> For a one-off export, a standalone `--target <dir>` (which is then itself the
> root) also works. `--root` + `--subpath` is the pattern for the shared root.

### Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--project` | *(required)* | Which Claude project to export. |
| `--root` | *(this or `--target`)* | Shared document-root directory; this project lands under `--subpath`. |
| `--subpath` | `""` | Relative path within `--root` for this project, e.g. `claude/thane-ai-agent`. |
| `--target` | *(this or `--root`)* | Standalone directory that is itself the document root (alternative to `--root`). |
| `--root-name` | basename of `--root`/`--target` | Override the Thane root name in frontmatter (`managed_root`) and `<root>:` refs. |
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
- an **Overview** (time span, every branch touched, working dir, activity counts,
  tools used, linked PRs, Claude Code version);
- a **Conversation** section: human prompts (block-quoted), assistant prose,
  thinking (collapsible), tool calls humanized to one line with their output
  folded into `<details>`, PR links, and `⎇` markers wherever the working branch
  switches so interleaved cross-branch work stays legible.

Chronologically adjacent sessions are linked at the foot of each document
(← previous / next →), giving the whole corpus a continuous spine across months.

### Forked sessions

Forking a session in the Claude UI writes a new transcript that **copies the
parent's conversation prefix verbatim** — and those copied records keep the
*parent's* session id; only the divergent tail carries the fork's own id (which
is the filename). The exporter handles this so forks don't pollute the corpus:

- the canonical session id comes from the **filename**, so each fork gets its
  own `conversation:` ref (not the parent's);
- the session is dated from its **divergence point** (first own message), so
  forks sort to when their work actually happened — not when the parent began;
- the copied prefix is **not repeated**; the document renders only the fork's
  own work, led by a fork note that links the parent and a `forked-from:<id>`
  source-ref (`> 🍴 Forked from [parent] — N earlier messages … not repeated here`).

A non-forked session has no inherited prefix and is unaffected. The detection is
data-driven (it only triggers when records actually carry a different id), so it
degrades gracefully if Claude Code's fork format ever changes.

## Idempotency / sync model

Output is a deterministic function of the transcripts, so re-running is a
push-sync of the project's subtree (each export is confined to its own
`<agent>/<project>/` directory):

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

Register the **one** shared root in Thane's config — not one per project. The
root key matches the root name (the `--root` basename you export with):

```yaml
paths:
  agentic-coding: ~/Thane/agentic-coding

doc_roots:
  agentic-coding:
    authoring: read_only        # this corpus is generated, not hand-edited
    git:
      enabled: true
      sign_commits: true        # if YOU commit the root with a trusted key
      verify_signatures: warn   # or: required
      signing_key: ~/.ssh/id_ed25519
```

Thane walks the whole root, so every project subtree (`claude/<project>/`,
`codex/<project>/`, …) is indexed under the single `agentic-coding` root. Export
each project with the same `--root`; only `--subpath` changes.

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
  `monthDir` / `sessionFileName` in `internal/export/export.go`, plus `Slug` in
  `internal/render/slug.go`.
- **Change how branch switches appear:** `writeBranchMarker` /
  `writeConversation` in `internal/render/session.go`.
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
