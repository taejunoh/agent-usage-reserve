# Codex Usage Reserve — initial release

## Approved product direction

An unofficial, local tool that adds a narrowly scoped usage-reserve policy to global Codex instructions. The user approved easy GitHub installation, Homebrew distribution, default 10% remaining, scheduled-task exemption, explicit continuation for the current task only, configurable threshold, and removal. Project and executable name: `codex-usage-reserve`.

## Delivery and architecture

A dependency-free Go executable for macOS, Windows and Linux, arm64 and amd64. Shell and PowerShell installers download a versioned GitHub Release, verify SHA-256, install in a user directory, and enable the policy. Homebrew installs the executable only; a separate `enable` command explicitly activates the policy. No service, API key, credential access, model calls or background polling.

CLI contract:

```text
codex-usage-reserve enable [--threshold 10] [--codex-home PATH]
codex-usage-reserve set 15 [--codex-home PATH]
codex-usage-reserve status [--codex-home PATH]
codex-usage-reserve disable [--codex-home PATH]
codex-usage-reserve --version
```

Threshold is an integer 1–99. A repeat `enable` preserves the configured threshold unless explicitly provided. `set` requires an enabled policy. `status` checks installed policy and target path; it does not fetch account usage or pretend to verify agent compliance. Home resolution: explicit flag, nonempty `CODEX_HOME`, then user home `.codex`.

## Instruction policy

Use concise English instructions so they work regardless of chat language. Before ordinary work and between major stages, use the available Codex account usage-limits tool (`get_usage_limits` or its namespaced equivalent). Remaining is 100 minus usedPercent, clamped to 0–100; null/missing windows are unavailable. Prefer rateLimitsByLimitId to legacy rateLimits. Check the limits applicable to the current task/model, not unrelated special-model buckets. At <= threshold, checkpoint safely and stop ordinary work; tell the user the measured window and remaining percentage, why work stopped, progress and how to resume. Actual scheduled runs are exempt; unrelated manual work is not. Explicit user continuation applies only to the specified/current task; a user-specified temporary threshold wins, otherwise the task may finish. Missing usage data is disclosed without inventing values or adding a new blocking rule. Policy is behavioral, not a hard account cap, and cannot guarantee exact remaining usage or scheduled-run success.

## File integrity

Prefer nonempty global `AGENTS.override.md`, otherwise `AGENTS.md`, in line with Codex precedence. An existing managed block in the inactive file is detected and reported rather than duplicated or silently left effective. Instructions are owned only between distinctive versioned comment markers. Preserve every unrelated byte including original CRLF and a missing final newline. Record any generated separator as part of the managed region so disable reverses it exactly. Preserve comments and unrelated text appended after installation. Refuse ambiguous/malformed/duplicate markers, symlink instruction targets and concurrent mutations. Back up pre-mutation bytes in the Codex directory with private permissions, write a same-directory temp file, preserve target permissions, close and atomically rename. Avoid lost updates by rechecking the original before replacement. A dedicated exclusive lock file serializes this tool; never delete someone else's lock. Missing directories are created for enable only. Disable is idempotent. Do not migrate or modify unrelated pre-existing reserve policies.

## Distribution

Repository destination prepared for `taejunoh/codex-usage-reserve`; no remote publication without a concrete final review. Version 0.1.0. Tagged GitHub workflow builds six archives plus `checksums.txt`, and publishes only on version tags. macOS/Linux installer defaults to `$HOME/.local/bin`; Windows defaults to a user-local bin path. Installation output shows the exact executable path and how to address PATH, and never silently modifies shell startup files. A download/hash failure never enables the policy. Fetch the latest release tag once, then pin asset and checksum downloads to that same tag. Verify an explicit version's format. Support local archive fixtures for meaningful installer tests without touching real user instructions.

Homebrew formula packaging must not contain fabricated hashes. Generate it from verified release artifacts; include generation script and instructions for a separate `homebrew-tap`. Brew availability is described as pending until that tap and release exist.

## Verification and scope

Go tests cover lifecycle, exact preservation, repeats, overrides, validation, malformed markers, symlinks, locking, backups and concurrent file edits. Shell installer test uses local HTTP or command stubs and a fake Codex home; exercise success and bad checksum with no mutation. CI tests on macOS/Linux/Windows and cross-builds release platforms. Locally run tests, vet, race tests where supported, all-platform builds, shell syntax/integration, and a temp-home CLI smoke test. Windows runtime behavior is CI-verified, not claimed as locally tested on macOS.

English README first with Korean companion; both show quick start, limitations, updates, disable versus binary uninstall, and local source use before public release. MIT license. Keep implementation small, no telemetry or credential scraping.
