# Codex Usage Reserve

`codex-usage-reserve` manages a small usage-reserve policy in your global Codex instructions. It is an unofficial local utility. It has no service, telemetry, API-key access, credential scraping, or background polling.

## Demo

![Actual CLI output in an isolated demo profile](docs/assets/demo.gif)

Actual CLI output · isolated demo profile. The demo configures agent guidance; it is not an enforced usage cap. Temporary paths are shortened for readability.

## Quick start

Install with Homebrew on macOS or Linux:

```sh
brew install taejunoh/tap/codex-usage-reserve
codex-usage-reserve enable
```

Homebrew installs the executable only; `enable` activates the policy. To install and enable in one step instead, use the direct installer on macOS/Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/taejunoh/codex-usage-reserve/main/install.sh | sh
```

On Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/taejunoh/codex-usage-reserve/main/install.ps1 | iex
```

Direct installers verify the release checksum and executable version, install to a user directory, then automatically run `enable`. They do not use `sudo` or modify shell startup files. Unix installs to `~/.local/bin`; Windows installs to `%LOCALAPPDATA%/Programs/codex-usage-reserve/bin`. Add the printed Windows directory to your user `PATH`, or run the printed `$env:Path = "...;$env:Path"` command for the current PowerShell session.

Installing and running the released executable requires no Go, Node, Python, or API keys.

## Update

To update a direct install, rerun the same installer for your platform. It runs `enable` without a threshold, preserving your custom threshold.

```sh
curl -fsSL https://raw.githubusercontent.com/taejunoh/codex-usage-reserve/main/install.sh | sh
```

On Windows, rerun the PowerShell installer:

```powershell
irm https://raw.githubusercontent.com/taejunoh/codex-usage-reserve/main/install.ps1 | iex
```

For Homebrew, update the tap and formula, check the policy, and enable it if needed:

```sh
brew update
brew upgrade taejunoh/tap/codex-usage-reserve
codex-usage-reserve status
codex-usage-reserve enable  # if status says disabled; keeps the current threshold
```

## Commands

```text
codex-usage-reserve enable [--threshold 10] [--codex-home PATH]
codex-usage-reserve set 15 [--codex-home PATH]
codex-usage-reserve status [--codex-home PATH]
codex-usage-reserve disable [--codex-home PATH]
codex-usage-reserve --version
```

Thresholds are whole percentages from 1 to 99. Repeating `enable` without `--threshold` keeps your current threshold. `set` changes an enabled policy. `status` reports the managed policy and its target; it does not check account usage or verify model behavior.

The tool updates only its marked policy block in the effective global `AGENTS.override.md` or `AGENTS.md` under your Codex home. It keeps a private backup of the original file in that directory before each mutation. `disable` removes the managed block while preserving unrelated text. Before replacement, the tool checks whether the target changed during its operation; this catches many external edits but cannot serialize unrelated editors or guarantee detection of every concurrent write. Disabling the policy and uninstalling the executable are separate: run `disable`, then remove the binary or run `brew uninstall taejunoh/tap/codex-usage-reserve`.

### Backup recovery

Prefer `codex-usage-reserve disable` to remove the managed block. Backups are named `.codex-usage-reserve-backup-*` in the Codex home directory. Find them on macOS/Linux with:

```sh
codex_home=${CODEX_HOME:-"$HOME/.codex"}
find "$codex_home" -type f -name '.codex-usage-reserve-backup-*' -print
```

Inspect the selected backup and current target first. A full restore replaces the entire instruction file and can erase edits made after that backup. Make a copy of the current target before an intentional full restore; choose the actual active target (`AGENTS.md` or `AGENTS.override.md`) printed by `status`.

```sh
backup='/full/path/to/.codex-usage-reserve-backup-...'
target="$codex_home/AGENTS.md" # use AGENTS.override.md when that is the active target
diff -u "$target" "$backup" || true
cp -p "$target" "$target.before-restore"
cp -p "$backup" "$target" # only when you intend to restore the complete file
```

On Windows PowerShell, list backups and review the selected backup and target:

```powershell
$codexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $HOME '.codex' }
Get-ChildItem -Force -File $codexHome -Filter '.codex-usage-reserve-backup-*' |
  Sort-Object LastWriteTime -Descending
```

```powershell
$backup = 'C:\full\path\to\.codex-usage-reserve-backup-...'
$target = Join-Path $codexHome 'AGENTS.md' # use AGENTS.override.md when that is the active target
Get-FileHash -LiteralPath $backup, $target
Copy-Item -LiteralPath $target -Destination "$target.before-restore"
Copy-Item -LiteralPath $backup -Destination $target -Force # only for an intentional full-file restore
```

Copy only the bytes you intend to restore if you need to preserve later edits.

## Scope and limits

The policy guides Codex to check the available account usage limits before ordinary work and between major stages. Actual scheduled runs are exempt. An explicit request to continue applies only to the named task, or the current task if none is named. A user-specified temporary threshold takes precedence.

This is behavioral guidance, not a hard account cap. It can only use usage data and tools available to the current Codex desktop or task environment; it does not promise that every CLI or scheduled run has those tools. Missing usage data is reported without guessing. It does not include any user's custom model-selection or delegation policies.

## For developers

### Build and enable from source

Go 1.23 or newer is required to build from source.

```sh
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/codex-usage-reserve" ./cmd/codex-usage-reserve
"$HOME/.local/bin/codex-usage-reserve" enable
```

The default threshold is 10% remaining. The command prints the instruction file it changed. If `$HOME/.local/bin` is not on `PATH`, add it in your shell configuration yourself.

To prepare a release, `scripts/build-release.sh 0.1.0` creates six platform archives and their real SHA-256 entries in `dist/`. The tagged GitHub Actions workflow uses those files to publish the Release.

Generate a Homebrew formula from the release checksum manifest in a separate tap checkout:

```sh
python3 scripts/generate-formula.py 0.1.0 dist/checksums.txt ../homebrew-tap/Formula/codex-usage-reserve.rb
```

The generator requires real checksums for all six release archives and embeds the macOS/Linux asset URLs and hashes. Review and publish the generated formula in the separate tap repository.
