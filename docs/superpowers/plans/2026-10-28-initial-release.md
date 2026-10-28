# Codex Usage Reserve Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development to implement this plan. Steps use checkbox syntax for tracking.

**Goal:** Deliver an installable, reversible 10% usage-reserve policy manager ready for GitHub distribution.

**Architecture:** One Go executable owns only a managed region in effective global AGENTS instructions. Installers obtain version-pinned, checksum-verified binaries and then invoke enable. Release automation and a formula generator support distribution without embedding unreleased hashes.

**Tech Stack:** Go standard library, POSIX shell, PowerShell, GitHub Actions, Homebrew Ruby formula generator.

---

### Task 1: Policy lifecycle and CLI

**Files:** `go.mod`, `cmd/codex-usage-reserve/main.go`, `internal/policy/*.go`, corresponding Go tests.

- [ ] Write behavioral tests for `enable`, repeat-enable preserving threshold, `set 15`, `status`, `disable`, invalid 0/100/noninteger thresholds and unsupported commands; execute them and observe failure before implementation.
- [ ] Implement the CLI contract and all file-integrity/policy requirements from the design. Version defaults to `dev` and supports `-ldflags -X main.version=0.1.0`.
- [ ] Test byte-exact round trips for empty, LF, CRLF, Unicode and no-final-newline instruction files; add text after enabling and prove disable preserves it.
- [ ] Test override precedence, inactive managed blocks, backup data and permissions, duplicate/malformed sentinels, symlinks, held locks and pre-replace external edits.
- [ ] Run `go test ./...` and `go vet ./...`, format, self-review, and report a consolidated result. Do not touch real user instructions.

### Task 2: Distribution, documentation and installer verification

**Files:** `install.sh`, `install.ps1`, `scripts/*`, `.github/workflows/*`, `README.md`, `README.ko.md`, `LICENSE`, `.gitignore`.

- [ ] Write shell installer integration checks using a fixture archive, exact checksums, isolated install path and fake Codex home. Test failed hash does not enable or replace a working install. Record a failing check before implementation.
- [ ] Build shell and PowerShell installers for `taejunoh/codex-usage-reserve` releases. Pass explicit Codex-home options to the installed binary only when supplied; otherwise honor CODEX_HOME. Auto-enable on direct installation and clearly report PATH instructions. Defaults are 10%; repeat installation preserves existing configured threshold.
- [ ] Add CI (`go test ./...`, `go vet ./...`) on Linux/macOS/Windows, plus shell installer integration on Unix. Tagged releases produce versioned archives for darwin/linux/windows, amd64/arm64, and SHA-256 checksums using a shared build script with no third-party Go dependencies.
- [ ] Implement a formula generator that reads local checksums and writes a Homebrew formula with actual artifact URLs/hashes, executable-only installation, caveats for `enable`, and a temp-home test. Never supply a dummy hash or claim the tap is live.
- [ ] Write concise English and Korean README quick starts, source-use instructions, current release availability, lifecycle commands, recovery/backups, exact scope and limitations, and proposed tap publication steps. Add MIT license.
- [ ] Run `sh -n install.sh`, shell integration checks and locally feasible generator/script checks. Report Windows validation limits accurately.

### Task 3: Independent review, repair and final verification

- [ ] Review spec coverage first, then file-mutation/installer/release correctness. Consolidate findings into a bounded review; fix all important defects before completion.
- [ ] Run Go tests/vet, race tests, six cross-builds, shell integration and formula generation/syntax checks on the final tree.
- [ ] Build local macOS binary and complete enable/status/set/disable in a disposable home, verifying the original file hash is restored.
- [ ] Package a source archive and local executable in outputs. Commit the completed local project on its implementation branch.
- [ ] Report what works and what has not been published or run on Windows. Present the concrete GitHub/release/tap publication step for user review.
