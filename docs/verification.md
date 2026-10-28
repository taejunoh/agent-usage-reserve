# Initial release verification

Date: 2026-10-28. Version: 0.1.0. Local platform: macOS arm64. Build toolchain: Go 1.27.3; source minimum: Go 1.23; Go dependencies: standard library only.

## Passed locally

- `go test -count=1 -race ./...` and `go vet ./...`.
- CLI lifecycle in an isolated Codex home: enable at 10%, set to 15%, repeated enable retains 15%, status, disable. Original instruction bytes and SHA-256 were restored, including CRLF and no final newline. A second cycle preserved instructions appended by the user after installation.
- Core regression tests for backups, file modes, malformed and duplicate markers, symlinks, override precedence, lock ownership, external edits, file replacement, home precedence and version/help handling.
- Shell installer integration: default/latest and explicit versions, pinned asset downloads, successful enable, bad checksum preserving the previous executable and never enabling.
- Shell syntax checks for installer, build and integration scripts; GitHub workflow YAML parsing.
- Six release cross-builds: macOS/Linux/Windows, amd64/arm64. Independently verified each archive's SHA-256 and actual executable machine type, and exact archive members (executable and MIT LICENSE). macOS metadata sidecars are excluded.
- Homebrew formula generation from the actual final checksum manifest and Ruby syntax validation.
- Independent specification and bounded code-quality reviews. Material Windows snapshot-identity and release repository-selection findings were corrected and reviewed again.

## Execution boundaries

Windows and Linux executables were cross-built, not executed locally. The Windows Go CI job is prepared, but no remote CI result is claimed before repository publication. The PowerShell installer was reviewed statically; its runtime still needs Windows verification. Homebrew formula syntax was checked; an actual `brew install` from a published tap has not yet run.

The executable manages an instruction policy; it does not query usage directly, enforce an account cap, promise exact reserve percentages, or guarantee scheduled-task completion. The account usage-limits tool must be available to the Codex chat following the policy. Other editors can still race in the final snapshot-check/rename gap; detection is best effort.

All mutation tests used disposable homes. The user's actual global instructions were not modified by this project implementation.

## Publication readiness

Local source, release assets, checksum manifest, direct installers, CI/release workflows, and the Homebrew formula generator are ready. Repository visibility and remote publication are a separate user decision. The READMEs clearly distinguish prepared installation commands from a live public release.
