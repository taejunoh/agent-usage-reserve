# Agent Usage Reserve

**Keep some of your Codex or Claude plan usage in reserve for scheduled tasks.**

Once installed, Codex and Claude check how much of your plan usage is left (the 5-hour limit and the weekly limit) before they start ordinary work and between major steps. When 10% or less is left, they save their progress, stop, and tell you:

- which limit is low and how much is left,
- what they finished so far,
- how to pick up again.

Your scheduled tasks still run, so the remaining usage stays available for them. If you want to finish the current task anyway, just tell the agent to continue.

[한국어 설명](README.ko.md)

![Actual CLI output in an isolated demo profile](docs/assets/demo.gif)

Actual CLI output in an isolated demo profile with both agents. Temporary paths are shortened.

> This is guidance the agent follows, not a hard limit on your account. It runs entirely on your computer: no account login, API key, server, or tracking.

## Where it works

| Agent | Works in | Notes |
| --- | --- | --- |
| **Claude** | The **Code** tab of the Claude desktop app | The terminal `claude` command has no way to read usage, so there the agent only says usage can't be checked. |
| **Codex** | Codex apps and environments that show your usage limits | Some command-line or scheduled runs may not show usage. |

## Install

Pick **one** of these.

**Mac or Linux, one command** (installs and turns the reserve on):

```sh
curl -fsSL https://raw.githubusercontent.com/taejunoh/agent-usage-reserve/main/install.sh | sh
```

**Windows**, in PowerShell (installs and turns the reserve on):

```powershell
irm https://raw.githubusercontent.com/taejunoh/agent-usage-reserve/main/install.ps1 | iex
```

**Homebrew** users on Mac or Linux (two commands: install, then turn on):

```sh
brew install taejunoh/tap/agent-usage-reserve
agent-usage-reserve enable
```

### What happens during install

- If you use only Codex or only Claude, the reserve is turned on for it at 10%.
- If you use both, you are asked once:

  ```text
  Found Codex and Claude. Apply the usage reserve to:
    1) Both
    2) Claude only
    3) Codex only
  Choose [1]:
  ```

  Press Enter for both.
- If the installer can't ask (for example in an automated setup), it changes nothing and prints the exact command to run instead, such as `agent-usage-reserve enable --only all`.
- If neither Codex nor Claude is installed yet, nothing changes. Install one of them first, then run `agent-usage-reserve enable`.

The installers check the download's checksum, install only into your user folder (`~/.local/bin` on Mac/Linux, `%LOCALAPPDATA%\Programs\agent-usage-reserve\bin` on Windows), and never ask for an administrator password. If the installer prints a line about `PATH`, follow it so you can type `agent-usage-reserve` directly.

## Everyday commands

| To do this | Run |
| --- | --- |
| See what is turned on | `agent-usage-reserve status` |
| Turn it on (or add the other agent) | `agent-usage-reserve enable` |
| Change the reserve to 15% | `agent-usage-reserve set 15` |
| Turn it off | `agent-usage-reserve disable` |
| Show the version | `agent-usage-reserve --version` |

The reserve can be any whole number from 1 to 99, and Codex and Claude can each have their own. To change only one agent, add `--only claude` or `--only codex`, for example `agent-usage-reserve set 20 --only codex`.

`status` shows what is installed in your instruction files. It doesn't look up your actual account usage.

## Update

Run the same install command again. Your current reserve settings are kept.

With Homebrew:

```sh
brew update
brew upgrade taejunoh/tap/agent-usage-reserve
```

## Uninstall

1. Turn the reserve off: `agent-usage-reserve disable`
2. Remove the program: `brew uninstall taejunoh/tap/agent-usage-reserve` for Homebrew, or otherwise delete the `agent-usage-reserve` file from the install folder above.

Do step 1 first. Removing the program alone leaves the reserve in your instruction files.

## Upgrading from codex-usage-reserve

This tool used to be called `codex-usage-reserve` and only supported Codex. It now supports Claude too.

1. Update with the same method you installed with:
   - **Homebrew:** `brew update`, then `brew upgrade taejunoh/tap/agent-usage-reserve`. Homebrew moves the old name to the new one automatically.
   - **Install command:** run the new install command above. The old install command also still works. Either one removes the old `codex-usage-reserve` program from the folder it installs into, and warns if another copy is still around.
2. Run `agent-usage-reserve enable` once. It keeps your Codex setting and, if you also use Claude, asks `Also enable for Claude? [Y/n]`.

Your existing Codex reserve keeps working the whole time.

## How it works and what it changes

The tool adds one clearly marked section to a single instruction file per agent, and changes nothing else:

- **Claude:** `CLAUDE.md` in your Claude folder (`~/.claude`, or `CLAUDE_CONFIG_DIR` if you set it)
- **Codex:** `AGENTS.md` in your Codex folder (`~/.codex`, or `CODEX_HOME` if you set it). If you have a non-empty `AGENTS.override.md`, that file is used instead, because Codex reads it first.

Before every change it saves a private backup of the file in the same folder. `disable` removes only its section and leaves the rest of your text exactly as it was.

The policy details:

- Scheduled task runs are exempt. Ordinary manual work stops at the reserve.
- "Continue" applies only to the task you name (or the current one). You can also give a lower temporary reserve for that task, such as "continue down to 3%".
- Subagents skip the check; the main session handles it.
- If usage can't be read, the agent says so instead of guessing.

## Advanced

### Options

```text
agent-usage-reserve enable  [--threshold 10] [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve set 15  [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve status  [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve disable [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve --version
```

- `--only` picks agents without asking. `--codex-home` / `--claude-home` point at a different folder, and also select that agent.
- An agent counts as found when its folder exists. `enable` creates the folder for an agent you select explicitly.
- Running `enable` again keeps agents that are already on and their reserves; `--threshold` changes them.
- `set` and `disable` without `--only` act on every agent that is on.
- When a choice is needed but there is no terminal to ask in, or no agent is found, `enable` changes nothing and exits with status 3.
- Before replacing a file, the tool checks whether something else changed it during the operation. This catches many outside edits but can't catch every simultaneous write.

### Restoring a backup

Use `agent-usage-reserve disable` to remove the section. Restore a backup only if you need the whole file back as it was.

Backups are named `.agent-usage-reserve-backup-*` and stay in each agent's folder. Backups made before the rename are named `.codex-usage-reserve-backup-*` and are kept as they are.

A full restore replaces the entire file, so anything you edited after that backup is lost. Restore a backup only into the file of the folder it is in. For Codex, use the file that `status` shows (`AGENTS.md` or `AGENTS.override.md`).

On Mac/Linux:

```sh
codex_home=${CODEX_HOME:-"$HOME/.codex"}
claude_home=${CLAUDE_CONFIG_DIR:-"$HOME/.claude"}
find "$codex_home" "$claude_home" -type f -name '.agent-usage-reserve-backup-*' -print
```

```sh
backup='/full/path/to/.agent-usage-reserve-backup-...'
target="$codex_home/AGENTS.md" # use AGENTS.override.md when that is the active Codex target
# For a Claude backup: target="$claude_home/CLAUDE.md"
diff -u "$target" "$backup" || true
cp -p "$target" "$target.before-restore"
cp -p "$backup" "$target" # only when you intend to restore the complete file
```

On Windows PowerShell:

```powershell
$codexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $HOME '.codex' }
$claudeHome = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $HOME '.claude' }
Get-ChildItem -Force -File $codexHome, $claudeHome -Filter '.agent-usage-reserve-backup-*' |
  Sort-Object LastWriteTime -Descending
```

```powershell
$backup = 'C:\full\path\to\.agent-usage-reserve-backup-...'
$target = Join-Path $codexHome 'AGENTS.md' # use AGENTS.override.md when that is the active Codex target
# For a Claude backup: $target = Join-Path $claudeHome 'CLAUDE.md'
Get-FileHash -LiteralPath $backup, $target
Copy-Item -LiteralPath $target -Destination "$target.before-restore"
Copy-Item -LiteralPath $backup -Destination $target -Force # only for an intentional full-file restore
```

To keep later edits, copy over only the part you want back instead of the whole file.

## For developers

The released program needs no Go, Node, Python or API key. Building from source needs Go 1.23 or newer:

```sh
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/agent-usage-reserve" ./cmd/agent-usage-reserve
"$HOME/.local/bin/agent-usage-reserve" enable
```

If `$HOME/.local/bin` is not on `PATH`, add it in your shell configuration yourself.

To prepare a release, `scripts/build-release.sh 0.2.0` creates six platform archives and their SHA-256 entries in `dist/`. Pushing a `v*` tag runs the release workflow, which publishes them. A release is marked Latest only when its commit is already on `main`, because the install commands read `main` and the latest release together.

Generate the Homebrew formula from the release checksums in a separate checkout of the tap. `--renames` also writes `formula_renames.json`, which maps the old formula name to the new one:

```sh
python3 scripts/generate-formula.py 0.2.0 dist/checksums.txt ../homebrew-tap/Formula/agent-usage-reserve.rb --renames ../homebrew-tap/formula_renames.json
```

The generator requires real checksums for all six archives. Review and publish the generated formula in the tap repository.

To regenerate the demo GIF and transcript, build the binary and run the script (needs Python with Pillow and a monospace font):

```sh
go build -o /tmp/agent-usage-reserve ./cmd/agent-usage-reserve
AGENT_USAGE_RESERVE_DEMO_BINARY=/tmp/agent-usage-reserve python3 scripts/generate-demo.py
```
