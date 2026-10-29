# Agent Usage Reserve Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename `codex-usage-reserve` to `agent-usage-reserve` and manage the usage-reserve policy for Codex and Claude Code from one executable, per `docs/superpowers/specs/2026-10-29-agent-usage-reserve-design.md`.

**Architecture:** The existing file-integrity engine becomes agent-agnostic and is driven by small agent definitions (Codex, Claude). A CLI layer resolves homes, detects agents, selects targets (prompting only when the spec says so) and prints one line per agent. Installers, release scripts and docs move to the new name and clean up the old executable.

**Tech Stack:** Go standard library (Go 1.23), POSIX shell, PowerShell, GitHub Actions, Homebrew Ruby formula generator.

## Global Constraints

- Executable, archives, Go module and repository: `agent-usage-reserve`; module path `github.com/taejunoh/agent-usage-reserve`. Version `0.2.0`.
- No third-party Go dependencies. No service, telemetry, API keys, credential access, model calls or background polling.
- CLI output English. Threshold integer 1–99, default 10, stored per agent in its marker.
- Codex policy text unchanged except markers. Claude policy text exactly as in the spec, including the subagent sentence.
- New marker: `<!-- agent-usage-reserve:managed:v1:begin agent=<codex|claude> threshold=N padding=P -->` … `<!-- agent-usage-reserve:managed:v1:end -->`. Legacy marker `codex-usage-reserve:managed:v1` recognized in Codex files only.
- Backups `.agent-usage-reserve-backup-*`, temp files `.agent-usage-reserve-write-*`, lock `.agent-usage-reserve.lock`, each in the agent's own home.
- Every initial-release file-integrity guarantee stays (byte-exact preservation, CRLF, missing final newline, recorded padding, symlink refusal, private backups, atomic same-directory replace, permission preservation, external-edit detection, never deleting someone else's lock).
- Tests never touch real `~/.codex` or `~/.claude`: always pass explicit homes or set `HOME`, `CODEX_HOME`, `CLAUDE_CONFIG_DIR` to temp dirs.
- No push, tag, release, repository rename or tap publication. Those need user approval later.

---

### Task 1: Agent-agnostic engine, agent definitions, legacy marker

**Agent:** deep-reasoner (careful refactor of integrity-critical code).

**Files:**
- Create: `internal/reserve/file.go` (engine moved from `reserve.go`), `internal/reserve/agents.go`, `internal/reserve/agents_test.go`
- Modify: `internal/reserve/reserve.go` (keeps only `Run`, `help`, `parseThreshold` until Task 2), `internal/reserve/reserve_test.go`

**Interfaces (produced, used by Task 2):**

```go
type agent struct {
	name, label   string   // "codex"/"Codex", "claude"/"Claude"
	homeFlag      string   // "codex-home", "claude-home"
	homeEnv       string   // "CODEX_HOME", "CLAUDE_CONFIG_DIR"
	defaultDir    string   // ".codex", ".claude"
	targets       []string // later entries win when nonempty: codex {"AGENTS.md","AGENTS.override.md"}, claude {"CLAUDE.md"}
	legacy        bool     // true only for codex: also parse codex-usage-reserve markers
	note          string   // claude: "works in the Claude desktop app's Code tab"; codex: ""
	policy        func(threshold int) string // body between markers, no markers
}
var agents = []agent{claudeAgent, codexAgent} // fixed processing/printing order

type result struct {
	agent     agent
	enabled   bool
	threshold int
	target    string // absolute path of the effective file
	changed   bool
}

// apply runs "enable", "set" or "disable" for one agent home. threshold 0 means keep/default.
func apply(a agent, home, command string, threshold int) (result, error)
// inspect reports state without locking or writing; a missing home is disabled.
func inspect(a agent, home string) (result, error)
```

- [ ] Write failing tests in `agents_test.go`: Claude enable/set/disable byte-exact round trip on `""`, `"hello"`, `"hello\n"`, CRLF; Claude marker carries `agent=claude` and contains the subagent sentence; a codex marker inside `CLAUDE.md` (and vice versa) is refused; legacy marker in `AGENTS.md` → `inspect` enabled with its threshold, `apply enable` rewrites to the new marker keeping threshold and padding and preserving surrounding bytes, `apply set 20` likewise, `apply disable` removes it byte-exactly; legacy plus new block in one file refused as duplicate; legacy marker in `CLAUDE.md` refused. Run `go test ./internal/reserve/` and confirm failures.
- [ ] Move the engine into `file.go` parameterized by `agent`: `findBlock(data []byte, a agent)` counts new and (if `a.legacy`) legacy prefixes, requires exactly one begin/end pair overall, parses `agent=` and rejects a mismatch, and returns `block{start, end, threshold, padding int; legacy bool}`. Generalize `readTargets` to `a.targets` with the existing inactive-file rule. Rename backup/temp/lock prefixes. Move Codex text into `agents.go` verbatim; add the Claude text from the spec.
- [ ] Keep `Run` behaving as today by delegating to `apply`/`inspect` with `codexAgent`, so every existing test in `reserve_test.go` passes unchanged except marker/backup/lock name assertions, which are updated to the new names.
- [ ] Run `gofmt -l .`, `go vet ./...`, `go test ./...`, `go test -race ./...`; all pass.
- [ ] Commit: `refactor: make reserve engine agent-agnostic and add Claude policy`.

### Task 2: Multi-agent CLI, selection prompt, rename of module and command

**Agent:** deep-reasoner.

**Files:**
- Create: `internal/reserve/cli.go`, `internal/reserve/cli_test.go`, `cmd/agent-usage-reserve/main.go`
- Delete: `cmd/codex-usage-reserve/`
- Modify: `go.mod`, `internal/reserve/reserve.go` (remove what moved to `cli.go`), `internal/reserve/reserve_test.go` (helper passes `--only codex --codex-home`)

**Interfaces:**
- Consumes: `agent`, `agents`, `apply`, `inspect`, `result` from Task 1.
- Produces:

```go
// Run executes a CLI invocation. in supplies prompt answers; interactive reports
// whether a prompt may be shown. Callers map errors to exit codes.
func Run(args []string, in io.Reader, out io.Writer, interactive bool, version string) error

var ErrNeedsChoice = errors.New("...") // prompt needed but not interactive; message lists the exact --only commands
var ErrNoAgent     = errors.New("...") // nothing detected and no --only
```

`main.go`: `interactive` is true when `os.Stdin.Stat()` has `os.ModeCharDevice`. Exit 3 for `errors.Is(err, ErrNeedsChoice) || errors.Is(err, ErrNoAgent)`, exit 1 for other errors; errors print as `agent-usage-reserve: <message>` to stderr.

- [ ] Write failing tests in `cli_test.go` (temp dirs for both homes, `CODEX_HOME`/`CLAUDE_CONFIG_DIR`/`HOME` set to temp via `t.Setenv`): no agent detected → `ErrNoAgent`, nothing created; only Claude dir → enabled without prompt; both dirs, interactive, inputs `"\n"`, `"1\n"`, `"2\n"`, `"3\n"` → both / Claude / Codex; `"x\n2\n"` → Claude; `"x\nx\nx\n"` → error, no files changed; both dirs, not interactive → `ErrNeedsChoice`, no files changed, message contains `--only all`, `--only claude`, `--only codex`; Codex enabled + Claude dir newly present → `Also enable for Claude? [Y/n]:` with `"\n"`/`"y\n"` enabling and `"n\n"` leaving it disabled, Codex untouched either way; repeat `enable` with both enabled → no prompt, thresholds kept; `--only claude` with missing dir creates it; `--only` invalid value rejected; `set 15` changes all enabled, `set 15 --only claude` only Claude, `set` on disabled selection errors; `disable` without `--only` disables all, idempotent; `status` lines exactly `✓ Claude  10%  <path>  (works in the Claude desktop app's Code tab)`, `✓ Codex   10%  <path>`, `– Codex   disabled  <path>`, followed by `Installed policy only; account usage and compliance are not checked.`; status with nothing detected prints `No Codex or Claude home found.`; `--help` and `--version`. Confirm they fail.
- [ ] Implement `cli.go` per the spec's selection rules. Flags: `--only claude|codex|all`, `--threshold` (enable only), `--codex-home`, `--claude-home`; an explicit home flag selects that agent like `--only`. Home resolution: flag, nonempty env, `~/<defaultDir>`, made absolute. Process agents in `agents` order; stop at the first error after printing lines for agents already applied. Help text documents all of this in the style of the current `help`.
- [ ] Rename module in `go.mod`, add `cmd/agent-usage-reserve/main.go`, delete the old command directory. Update `reserve_test.go` helper.
- [ ] Run `gofmt -l .`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and cross-build `GOOS=windows go build ./...`, `GOOS=linux go build ./...`; all pass.
- [ ] Smoke test in a temp `HOME` with both `.codex` and `.claude`: `printf '1\n' | go run ./cmd/agent-usage-reserve enable` is non-interactive under a pipe, so expect exit 3 and the `--only` guidance; then `enable --only all`, `status`, `set 15 --only claude`, `disable`, and confirm original file hashes are restored.
- [ ] Commit: `feat: manage Codex and Claude from agent-usage-reserve`.

### Task 3: Installers, release scripts, formula generator, CI

**Agent:** fast-worker.

**Files:** `install.sh`, `install.ps1`, `scripts/test-install.sh`, `scripts/test-install.ps1`, `scripts/build-release.sh`, `scripts/generate-formula.py`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`

**Interfaces:**
- Consumes: executable `agent-usage-reserve` with exit codes 0 / 3 / 1 from Task 2.
- Produces: archives `agent-usage-reserve_<version>_<os>_<arch>.tar.gz`, `checksums.txt`; installer env vars renamed from `CODEX_USAGE_RESERVE_*` to `AGENT_USAGE_RESERVE_*` (`_VERSION`, `_OS`, `_ARCH`, `_RELEASE_BASE_URL`, `_INSTALL_DIR`, `_CODEX_HOME`, `_CLAUDE_HOME`, `_DIST_DIR`).

- [ ] Extend `scripts/test-install.sh` first and confirm it fails: fixture archive named for the new binary; pre-seed `<install_dir>/codex-usage-reserve` (a stub that prints a version) and assert it is removed and reported; fake homes for both agents with `AGENT_USAGE_RESERVE_CODEX_HOME` and `AGENT_USAGE_RESERVE_CLAUDE_HOME` → installer passes both `--codex-home` and `--claude-home` and enables both non-interactively via those explicit flags; a run with no detectable agent → installer exits 0, prints `Installed (policy not enabled yet)` plus the guidance; bad checksum → exit nonzero, no binary replaced, no instruction file touched, old executable not removed.
- [ ] Update `install.sh`: `REPO=taejunoh/agent-usage-reserve`; new names; after installing, remove only `$install_dir/codex-usage-reserve` if it is a regular file, printing `Removed old codex-usage-reserve: <path>`; pass `--codex-home`/`--claude-home` only from `AGENT_USAGE_RESERVE_CODEX_HOME`/`AGENT_USAGE_RESERVE_CLAUDE_HOME` (never from `CODEX_HOME`/`CLAUDE_CONFIG_DIR`, which the binary reads itself; passing them as flags would force-select that agent and skip detection of the other); run `enable` with `</dev/tty` only when `( : </dev/tty ) 2>/dev/null` succeeds; exit status 0 → `Installed and enabled: <path>`, 3 → `Installed (policy not enabled yet): <path>` and exit 0, other → exit with that status. PATH guidance unchanged.
- [ ] Mirror in `install.ps1` and `scripts/test-install.ps1` (old file `codex-usage-reserve.exe` in the same bin dir; `$LASTEXITCODE` 3 handling; console stdin used directly).
- [ ] Update `build-release.sh`, `release.yml`, `ci.yml` (build path `./cmd/agent-usage-reserve`). Update `generate-formula.py`: `REPOSITORY`, archive names, `class AgentUsageReserve < Formula`, desc `Manage a local usage-reserve policy in Codex and Claude global instructions`, test block sets `CODEX_HOME` and `CLAUDE_CONFIG_DIR` to testpath dirs and runs `enable --only all`, `status`, `disable`; caveats name the new commands. Add `--renames PATH` option that writes `{"codex-usage-reserve": "agent-usage-reserve"}` to `formula_renames.json`.
- [ ] Run `sh -n install.sh`, `sh scripts/test-install.sh`, `scripts/build-release.sh 0.2.0` (into a temp `AGENT_USAGE_RESERVE_DIST_DIR`), `python3 scripts/generate-formula.py 0.2.0 <dist>/checksums.txt <tmp>/agent-usage-reserve.rb --renames <tmp>/formula_renames.json`, and `ruby -c` on the formula if Ruby is available. `pwsh` tests only if `pwsh` exists locally; otherwise state they are CI-only.
- [ ] Commit: `build: ship installers and releases as agent-usage-reserve`.

### Task 4: Documentation and demo

**Agent:** fast-worker.

**Files:** `README.md`, `README.ko.md`, `docs/verification.md`, `scripts/generate-demo.py`, `docs/assets/demo.gif`, `docs/assets/demo-transcript.json`

- [ ] Rewrite both READMEs for the new name with the same section order: quick start (brew and direct install with new URLs; `enable` prompt explained in one sentence), update, commands, backup recovery (new backup prefix; both homes), scope and limits (Claude works in the desktop app's Code tab only; terminal CLI has no usage tool; subagents skip the check; behavioral, not a hard cap), new short section "Existing codex-usage-reserve users" (brew upgrade migrates; direct installer removes the old executable; existing policy keeps working and is migrated on next enable/set), developer section with new paths and env vars. English and Korean must match in content.
- [ ] Update `generate-demo.py` to the new binary with a temp home containing both agents and `enable --only all`, `status`, `disable`. Regenerate the GIF and transcript if Pillow and a monospace font are available; otherwise leave the old assets, remove the GIF reference from both READMEs, and report it.
- [ ] Add a 0.2.0 entry to `docs/verification.md` recording only what was actually run locally.
- [ ] Commit: `docs: document agent-usage-reserve and migration`.

### Task 5: Review, repair and final verification

**Agent:** main session orchestrates; independent review via `/codex:adversarial-review`.

- [ ] Review spec coverage, then integrity of the engine refactor (diff `file.go` against the old `reserve.go` behavior), selection logic and installer exit handling. Fix every important defect.
- [ ] On the final tree run `gofmt -l .`, `go vet ./...`, `go test ./...`, `go test -race ./...`, six cross-builds via `build-release.sh`, `sh scripts/test-install.sh`, formula generation.
- [ ] Disposable-home end-to-end on macOS: seed `~/.codex/AGENTS.md` with a legacy `codex-usage-reserve` block and `~/.claude/CLAUDE.md` with user text; run `enable` under a pseudo-terminal (`script -q /dev/null ./agent-usage-reserve enable` with input `1`) to exercise the real prompt; verify migration, `status`, `disable`, and that `CLAUDE.md` returns to its original hash.
- [ ] Report what was verified, what is CI-only (Windows), and the pending outward-facing steps: repository rename, push, tag `v0.2.0`, tap formula plus `formula_renames.json` (note `../homebrew-tap` locally has only `Casks/`, so confirm the real tap location first).
