# Agent Usage Reserve — Claude support and rename

## Approved product direction

Rename `codex-usage-reserve` to `agent-usage-reserve` and manage the same usage-reserve policy for both Codex and Claude Code from one executable. The deciding criterion is that non-developers can use it without confusion: one install, one command, one status view. Version 0.2.0. All other principles of the initial release design stay: local only, no service, telemetry, API keys, credential access, model calls or background polling; dependency-free Go; behavioral policy, not a hard cap.

## CLI contract

```text
agent-usage-reserve enable  [--threshold 10] [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve set 15  [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve status  [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve disable [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve --version
```

Threshold is an integer 1–99, stored per agent in that agent's marker. CLI output stays English; Korean guidance lives in `README.ko.md`.

Home resolution per agent: explicit flag, then nonempty environment variable (`CODEX_HOME`, `CLAUDE_CONFIG_DIR`), then `~/.codex` / `~/.claude`. An agent is detected when its resolved home directory exists. An explicit home flag or `--only` naming an agent counts as selecting it; missing directories are created for `enable` only, as before.

### Target selection for `enable`

- `--only` given: use exactly those agents; no prompt.
- Agents already enabled are kept enabled without asking (a repeat `enable` without `--threshold` preserves each agent's threshold, as before).
- Of the detected agents not yet enabled: if none, nothing to ask; if exactly one and no agent is enabled yet, apply it without asking; if one new agent appears while another is already enabled, or two are detected and neither is enabled, ask.
- Prompt (default 1 on empty input; invalid input re-asks up to 3 times, then aborts without changes):

  ```text
  Found Codex and Claude. Apply the usage reserve to:
    1) Both
    2) Claude only
    3) Codex only
  Choose [1]:
  ```

  When only one new agent is in question, the prompt is a yes/no `Also enable for Claude? [Y/n]:`.
- No interactive terminal (stdin is not a character device) and a prompt would be needed: change nothing, exit nonzero, and print the exact `--only` commands to run.
- No agent detected and no `--only`: change nothing, exit nonzero, and say Codex or Claude was not found and to run again after installing one.

`set` and `disable` without `--only` act on every enabled agent. `disable` prints a line for every detected agent. `set` errors when the selected agent is not enabled. `disable` is idempotent. `status` lists every detected agent.

### Output

One line per agent, for example:

```text
✓ Claude  10%  ~/.claude/CLAUDE.md   (works in the Claude desktop app's Code tab)
✓ Codex   10%  ~/.codex/AGENTS.md
– Codex   disabled
```

The Claude line always carries the desktop-app note, because the terminal `claude` CLI exposes no usage tool to the model. Output still states that only the installed policy is checked, not account usage or compliance.

## Architecture

Split the current single file in `internal/reserve`:

- `file.go` — agent-agnostic file integrity engine: marker parsing, snapshot, backup, lock, atomic replace, concurrent-edit detection. Behavior and guarantees unchanged from the initial release (exact byte preservation, CRLF, missing final newline, recorded padding, symlink refusal, private backups, same-directory temp, permission preservation, exclusive lock never deleting someone else's lock).
- `agents.go` — agent definitions: name, home env var, default dir, ordered candidate target files, policy text function.
  - Codex: `AGENTS.override.md` when nonempty, else `AGENTS.md`; existing inactive-file detection kept.
  - Claude: `CLAUDE.md` only.
- `cli.go` — argument parsing, detection, selection, prompt. Reads answers from an injected `io.Reader` and decides interactivity through an injected check so tests do not need a terminal.

Marker format:

```text
<!-- agent-usage-reserve:managed:v1:begin agent=claude threshold=10 padding=1 -->
...
<!-- agent-usage-reserve:managed:v1:end -->
```

Backups `.agent-usage-reserve-backup-*` and the lock `.agent-usage-reserve.lock` live in each agent's own home directory.

## Instruction policy

Codex policy text is unchanged except for the marker.

Claude policy text follows the same structure with Claude's data shape:

> Before ordinary work and between major stages, check Claude plan usage with the `get_usage` tool (it may be namespaced, e.g. `mcp__ccd_session_mgmt__get_usage`). Use `plan.windows`; for each window, remaining percent is 100 minus `percentUsed`, clamped to 0–100. Applicable windows are the 5-hour limit, the all-models weekly limit, and a per-model weekly limit only when it matches the model doing the current work; ignore other model buckets and extra usage. If `plan.status` is not `ok` or a value is missing, usage is unavailable, not zero.
>
> If you are a subagent, skip this check; the main session handles it.

The remaining paragraphs match Codex: at or below threshold checkpoint and stop ordinary work, reporting window, remaining percentage, reason, progress and how to resume; actual scheduled task runs are exempt; explicit continuation applies only to the specified/current task, with a user-specified lower temporary threshold winning; unavailable data is disclosed without guessing; behavioral policy, not a hard cap.

## Migration from codex-usage-reserve

- The engine also recognizes the legacy marker `<!-- codex-usage-reserve:managed:v1:begin threshold=N padding=P -->` … `<!-- codex-usage-reserve:managed:v1:end -->` in Codex files. `status` reports it as enabled. `enable` and `set` rewrite it to the new marker, keeping threshold and padding. `disable` removes legacy and new blocks. A file containing both a legacy and a new block is refused as duplicate, as today.
- Legacy backups are left untouched.
- The old executable must not linger, because it cannot see the new marker and would add a second block:
  - Homebrew tap gains `formula_renames.json` mapping `codex-usage-reserve` to `agent-usage-reserve`, so `brew upgrade` migrates; the old formula file is removed.
  - Direct installers remove only a `codex-usage-reserve` executable in the same install directory they write to, and print that they did. On Windows, whose default install directory changed name, the installer also removes it from the previous default directory `%LOCALAPPDATA%\Programs\codex-usage-reserve\bin` when no custom install directory is set, and suggests removing that directory from PATH.
- The GitHub repository is renamed to `taejunoh/agent-usage-reserve`; GitHub redirects old web, git, raw and release URLs, so the old one-line install commands keep working.

## Distribution

`cmd/agent-usage-reserve`, release archives, checksums, installers, formula generator, CI and both READMEs move to the new name. Installers still verify checksums, install in a user directory, never use `sudo` or edit shell startup files, and run `enable` afterwards; under `curl | sh` the installer runs `enable` with stdin from `/dev/tty` when available so the prompt works, and PowerShell uses the console directly. When `enable` refuses because no terminal is available or no agent is found, the installer still reports the executable as installed and repeats `enable`'s guidance. A download or hash failure never enables the policy. READMEs gain a short "Existing codex-usage-reserve users" section.

Outward-facing steps — repository rename, push, tag, release publication and tap publication — each need explicit user approval at the time. This change covers local implementation and verification.

## Verification

Keep every existing Go test, retargeted to the engine. Add tests for: detection (none, one, both); selection (already enabled kept, one new agent yes/no, both); prompt input (empty, 1–3, invalid then valid, three invalid aborts); non-interactive refusal with no mutation; `--only`; full Claude lifecycle; legacy marker status, migration on enable/set, removal on disable, legacy plus new refused. Installer tests cover old-executable cleanup and that a bad checksum mutates nothing. CI runs on macOS, Linux and Windows; Windows runtime behavior is CI-verified, not claimed as locally tested.
