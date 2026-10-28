# Codex Usage Reserve Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development to implement this plan. Steps use checkbox syntax for tracking.

**Goal:** Deliver an installable, reversible 10% usage-reserve policy manager ready for GitHub distribution.

**Architecture:** One Go executable owns only a managed region in effective global AGENTS instructions. Installers obtain version-pinned, checksum-verified binaries and then invoke enable. Release automation and a formula generator support distribution without embedding unreleased hashes.

**Tech Stack:** Go standard library, POSIX shell, PowerShell, GitHub Actions, Homebrew Ruby formula generator.

---

### Task 1: Policy lifecycle and CLI

**Files:** `go.mod`, `cmd/codex-usage-reserve/main.go`, `internal/policy/*.go`, corresponding Go tests.

- [x] Write behavioral tests for `enable`, repeat-enable preserving threshold, `set 15`, `status`, `disable`, invalid 0/100/noninteger thresholds and unsupported commands; execute them and observe failure before implementation.
- [x] Implement the CLI contract and all file-integrity/policy requirements from the design. Version defaults to `dev` and supports `-ldflags -X main.version=0.1.0`.
- [x] Test byte-exact round trips for empty, LF, CRLF, Unicode and no-final-newline instruction files; add text after enabling and prove disable preserves it.
- [x] Test override precedence, inactive managed blocks, backup data and permissions, duplicate/malformed sentinels, symlinks, held locks and pre-replace external edits.
- [x] Run `go test ./...` and `go vet ./...`, format, self-review, and report a consolidated result. Do not touch real user instructions.

### Task 2: Distribution, documentation and installer verification

**Files:** `install.sh`, `install.ps1`, `scripts/*`, `.github/workflows/*`, `README.md`, `README.ko.md`, `LICENSE`, `.gitignore`.

- [x] Write shell installer integration checks using a fixture archive, exact checksums, isolated install path and fake Codex home. Test failed hash does not enable or replace a working install. Record a failing check before implementation.
- [x] Build shell and PowerShell installers for `taejunoh/codex-usage-reserve` releases. Pass explicit Codex-home options to the installed binary only when supplied; otherwise honor CODEX_HOME. Auto-enable on direct installation and clearly report PATH instructions. Defaults are 10%; repeat installation preserves existing configured threshold.
- [x] Add CI (`go test ./...`, `go vet ./...`) on Linux/macOS/Windows, plus shell installer integration on Unix. Tagged releases produce versioned archives for darwin/linux/windows, amd64/arm64, and SHA-256 checksums using a shared build script with no third-party Go dependencies.
- [x] Implement a formula generator that reads local checksums and writes a Homebrew formula with actual artifact URLs/hashes, executable-only installation, caveats for `enable`, and a temp-home test. Never supply a dummy hash or claim the tap is live.
- [x] Write concise English and Korean README quick starts, source-use instructions, current release availability, lifecycle commands, recovery/backups, exact scope and limitations, and proposed tap publication steps. Add MIT license.
- [x] Run `sh -n install.sh`, shell integration checks and locally feasible generator/script checks. Report Windows validation limits accurately.

### Task 3: Independent review, repair and final verification

- [x] Review spec coverage first, then file-mutation/installer/release correctness. Consolidate findings into a bounded review; fix all important defects before completion.
- [x] Run Go tests/vet, race tests, six cross-builds, shell integration and formula generation/syntax checks on the final tree.
- [x] Build local macOS binary and complete enable/status/set/disable in a disposable home, verifying the original file hash is restored.
- [x] Package the source archive and six platform executable archives in outputs; preserve the project on its implementation branch.
- [x] Report what works and what has not been published or run on Windows. Present the concrete GitHub/release/tap publication step for user review.
