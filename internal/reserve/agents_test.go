package reserve

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	beginFormat = "<!-- agent-usage-reserve:managed:v1:begin agent=%s threshold=%d padding=%d -->\n"
	endLine     = "<!-- agent-usage-reserve:managed:v1:end -->\n"
	legacyEnd   = "<!-- codex-usage-reserve:managed:v1:end -->\n"
)

// legacyBlock is the exact block codex-usage-reserve 0.1.0 wrote, kept verbatim
// so migration tests exercise real installed bytes.
func legacyBlock(threshold, padding int) string {
	return strings.Repeat("\n", padding) + fmt.Sprintf(`<!-- codex-usage-reserve:managed:v1:begin threshold=%d padding=%d -->
## Account usage reserve

Before ordinary work and between major stages, check available Codex account usage limits with get_usage_limits or its namespaced equivalent. Prefer rateLimitsByLimitId over legacy rateLimits. Check the actual rate-limit windows applicable to the current task and model; do not stop because of unrelated special-model buckets. For each available window, remaining percent is 100 minus usedPercent, clamped to 0–100. Null or missing values/windows are unavailable, not zero usage.

If any applicable window has %d%% or less remaining, checkpoint progress safely and stop ordinary work. Tell the user the measured window and remaining percentage, that work stopped to reserve usage for scheduled tasks, the current progress, and how to resume.

Actual scheduled task runs are exempt from this stopping threshold; unrelated manual work is not. If the user explicitly asks to continue, resume only the specified task (or the current task if unspecified). Follow a user-specified lower temporary threshold; otherwise that task may finish. This permission does not change the default threshold for other tasks.

If the usage tool or data is unavailable, disclose that fact without guessing remaining usage, claiming the threshold was reached, or inventing another blocking rule. This is a behavioral operating policy, not a hard account cap; it cannot guarantee an exact reserve or scheduled-task success.
<!-- codex-usage-reserve:managed:v1:end -->
`, threshold, padding, threshold)
}

// legacyBody is the policy text between the legacy marker lines.
func legacyBody(threshold int) string {
	block := legacyBlock(threshold, 0)
	return strings.TrimSuffix(block[strings.Index(block, "\n")+1:], legacyEnd)
}

// newBlock builds a current-format block directly from the marker contract.
func newBlock(name string, threshold, padding int, body string) string {
	return strings.Repeat("\n", padding) + fmt.Sprintf(beginFormat, name, threshold, padding) + body + endLine
}

// The two Claude-specific paragraphs, verbatim from the design spec.
const specClaudeUsageCheck = "Before ordinary work and between major stages, check Claude plan usage with the `get_usage` tool (it may be namespaced, e.g. `mcp__ccd_session_mgmt__get_usage`). Use `plan.windows`; for each window, remaining percent is 100 minus `percentUsed`, clamped to 0–100. Applicable windows are the 5-hour limit, the all-models weekly limit, and a per-model weekly limit only when it matches the model doing the current work; ignore other model buckets and extra usage. If `plan.status` is not `ok` or a value is missing, usage is unavailable, not zero.\n" +
	"\n" +
	"If you are a subagent, skip this check; the main session handles it.\n" +
	"\n"

const subagentSentence = "If you are a subagent, skip this check; the main session handles it."

func resolved(t *testing.T, dir string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func mustApply(t *testing.T, a agent, home, command string, threshold int) result {
	t.Helper()
	res, err := apply(a, home, command, threshold)
	if err != nil {
		t.Fatalf("%s %s: %v", a.name, command, err)
	}
	if res.agent.name != a.name {
		t.Fatalf("result for %q, want %q", res.agent.name, a.name)
	}
	return res
}

func mustInspect(t *testing.T, a agent, home string) result {
	t.Helper()
	res, err := inspect(a, home)
	if err != nil {
		t.Fatalf("%s inspect: %v", a.name, err)
	}
	if res.agent.name != a.name || res.changed {
		t.Fatalf("inspect result %+v", res)
	}
	return res
}

// assertRefused checks every operation refuses the file and leaves no trace.
func assertRefused(t *testing.T, a agent, home, path, content string) {
	t.Helper()
	if _, err := inspect(a, home); err == nil {
		t.Fatalf("%s inspect accepted %q", a.name, content)
	}
	for _, c := range []struct {
		command   string
		threshold int
	}{{"enable", 0}, {"enable", 15}, {"set", 20}, {"disable", 0}} {
		if _, err := apply(a, home, c.command, c.threshold); err == nil {
			t.Fatalf("%s %s accepted %q", a.name, c.command, content)
		}
	}
	if got := string(read(t, path)); got != content {
		t.Fatalf("changed refused file: %q", got)
	}
	leftovers, err := filepath.Glob(filepath.Join(home, ".agent-usage-reserve*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("left %v %v", leftovers, err)
	}
}

func TestAgentDefinitions(t *testing.T) {
	if len(agents) != 2 || agents[0].name != "claude" || agents[1].name != "codex" {
		t.Fatal("agents must be processed as Claude, then Codex")
	}
	for _, c := range []struct {
		a                           agent
		name, label, flag, env, dir string
		targets                     []string
		legacy                      bool
		note                        string
	}{
		{claudeAgent, "claude", "Claude", "claude-home", "CLAUDE_CONFIG_DIR", ".claude", []string{"CLAUDE.md"}, false, "works in the Claude desktop app's Code tab"},
		{codexAgent, "codex", "Codex", "codex-home", "CODEX_HOME", ".codex", []string{"AGENTS.md", "AGENTS.override.md"}, true, ""},
	} {
		a := c.a
		if a.name != c.name || a.label != c.label || a.homeFlag != c.flag || a.homeEnv != c.env || a.defaultDir != c.dir || a.legacy != c.legacy || a.note != c.note || strings.Join(a.targets, ",") != strings.Join(c.targets, ",") {
			t.Fatalf("agent %q definition %+v", c.name, a)
		}
	}
	for _, a := range agents {
		for _, threshold := range []int{1, 10, 99} {
			body := a.policy(threshold)
			if !strings.HasPrefix(body, "## Account usage reserve\n\n") || !strings.HasSuffix(body, ".\n") || strings.HasSuffix(body, "\n\n") {
				t.Fatalf("%s policy shape %q", a.name, body)
			}
			if strings.Contains(body, "usage-reserve:managed:") || strings.Contains(body, "\r") {
				t.Fatalf("%s policy contains marker text or CR", a.name)
			}
			if !strings.Contains(body, fmt.Sprintf("has %d%% or less remaining", threshold)) {
				t.Fatalf("%s policy lacks threshold %d", a.name, threshold)
			}
		}
	}
}

// Catches any drift in the Codex text, which must stay as initially released.
func TestCodexPolicyTextUnchanged(t *testing.T) {
	for _, threshold := range []int{1, 10, 99} {
		if got, want := codexAgent.policy(threshold), legacyBody(threshold); got != want {
			t.Fatalf("codex policy changed:\n%q\nwant\n%q", got, want)
		}
	}
}

// The Claude text is the spec's two paragraphs followed by the shared Codex rules.
func TestClaudePolicyText(t *testing.T) {
	for _, threshold := range []int{1, 10, 99} {
		codex := legacyBody(threshold)
		want := "## Account usage reserve\n\n" + specClaudeUsageCheck + codex[strings.Index(codex, "If any applicable window"):]
		if got := claudeAgent.policy(threshold); got != want {
			t.Fatalf("claude policy:\n%q\nwant\n%q", got, want)
		}
	}
}

// Catches lifecycle state errors and destructive rewriting of unrelated bytes in CLAUDE.md.
func TestClaudeLifecyclePreservesBytes(t *testing.T) {
	for _, original := range []string{"", "hello", "hello\n", "hello\r\nworld\r\n"} {
		t.Run(strings.ReplaceAll(original, "\n", "N"), func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "CLAUDE.md")
			write(t, path, []byte(original))
			target := filepath.Join(resolved(t, home), "CLAUDE.md")
			padding := 0
			if original != "" {
				padding = 2
				if strings.HasSuffix(original, "\n") {
					padding = 1
				}
			}
			res := mustApply(t, claudeAgent, home, "enable", 15)
			if !res.enabled || res.threshold != 15 || !res.changed || res.target != target {
				t.Fatalf("enable %+v", res)
			}
			installed := read(t, path)
			if string(installed) != original+newBlock("claude", 15, padding, claudeAgent.policy(15)) {
				t.Fatalf("installed %q", installed)
			}
			if res = mustInspect(t, claudeAgent, home); !res.enabled || res.threshold != 15 || res.target != target {
				t.Fatalf("inspect %+v", res)
			}
			if res = mustApply(t, claudeAgent, home, "enable", 0); res.changed || res.threshold != 15 || !bytes.Equal(installed, read(t, path)) {
				t.Fatalf("repeat enable changed policy: %+v", res)
			}
			if res = mustApply(t, claudeAgent, home, "set", 20); !res.enabled || !res.changed || res.threshold != 20 {
				t.Fatalf("set %+v", res)
			}
			if got := string(read(t, path)); got != original+newBlock("claude", 20, padding, claudeAgent.policy(20)) {
				t.Fatalf("set wrote %q", got)
			}
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.WriteString("user addition\r\n")
			if err = errors.Join(err, f.Close()); err != nil {
				t.Fatal(err)
			}
			if res = mustApply(t, claudeAgent, home, "disable", 0); res.enabled || !res.changed || res.target != target {
				t.Fatalf("disable %+v", res)
			}
			if got := string(read(t, path)); got != original+"user addition\r\n" {
				t.Fatalf("roundtrip %q", got)
			}
			if res = mustApply(t, claudeAgent, home, "disable", 0); res.enabled || res.changed {
				t.Fatalf("repeat disable %+v", res)
			}
			if res = mustInspect(t, claudeAgent, home); res.enabled {
				t.Fatalf("inspect after disable %+v", res)
			}
		})
	}
}

func TestClaudeMarkerInNewHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "claude")
	res := mustApply(t, claudeAgent, home, "enable", 0)
	if !res.enabled || res.threshold != 10 || !res.changed {
		t.Fatalf("enable %+v", res)
	}
	path := filepath.Join(home, "CLAUDE.md")
	content := string(read(t, path))
	if !strings.HasPrefix(content, "<!-- agent-usage-reserve:managed:v1:begin agent=claude threshold=10 padding=0 -->\n") || !strings.HasSuffix(content, endLine) {
		t.Fatalf("marker %q", content)
	}
	if !strings.Contains(content, subagentSentence) {
		t.Fatal("missing subagent sentence")
	}
	if _, err := os.Lstat(filepath.Join(home, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("Claude enable wrote a Codex file")
	}
	if runtime.GOOS != "windows" {
		for p, want := range map[string]os.FileMode{home: 0700, path: 0600} {
			info, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != want {
				t.Fatalf("%s mode %v, want %v", p, info.Mode().Perm(), want)
			}
		}
	}
}

func TestCrossAgentMarkersRefused(t *testing.T) {
	for _, c := range []struct{ owner, reader agent }{{codexAgent, claudeAgent}, {claudeAgent, codexAgent}} {
		t.Run(c.owner.name+"-marker-in-"+c.reader.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, c.reader.targets[0])
			content := "user text\n" + newBlock(c.owner.name, 10, 1, c.owner.policy(10))
			write(t, path, []byte(content))
			assertRefused(t, c.reader, home, path, content)
		})
	}
	t.Run("unknown-agent", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "CLAUDE.md")
		content := newBlock("gemini", 10, 0, "## Account usage reserve\n")
		write(t, path, []byte(content))
		assertRefused(t, claudeAgent, home, path, content)
	})
}

// Catches lost or rewritten bytes when migrating a codex-usage-reserve 0.1.0 block.
func TestLegacyMarkerMigration(t *testing.T) {
	const suffix = "later user text\r\n"
	for _, c := range []struct {
		prefix  string
		padding int
	}{{"", 0}, {"user text\r\n", 1}, {"user text", 2},
		// Recorded padding that differs from what enable would choose for the prefix.
		{"user text\n", 0}, {"user text\n\n", 1}, {"user text\n", 2}, {"", 1}} {
		legacy := c.prefix + legacyBlock(7, c.padding) + suffix
		setup := func(t *testing.T) (string, string) {
			home := t.TempDir()
			path := filepath.Join(home, "AGENTS.md")
			write(t, path, []byte(legacy))
			return home, path
		}
		t.Run(fmt.Sprintf("%q-padding%d", c.prefix, c.padding), func(t *testing.T) {
			t.Run("inspect", func(t *testing.T) {
				home, path := setup(t)
				res := mustInspect(t, codexAgent, home)
				if !res.enabled || res.threshold != 7 || res.target != filepath.Join(resolved(t, home), "AGENTS.md") {
					t.Fatalf("inspect %+v", res)
				}
				if string(read(t, path)) != legacy {
					t.Fatal("inspect changed legacy block")
				}
			})
			for _, op := range []struct {
				command         string
				threshold, want int
			}{{"enable", 0, 7}, {"enable", 15, 15}, {"set", 20, 20}} {
				t.Run(fmt.Sprintf("%s%d", op.command, op.threshold), func(t *testing.T) {
					home, path := setup(t)
					res := mustApply(t, codexAgent, home, op.command, op.threshold)
					if !res.enabled || !res.changed || res.threshold != op.want {
						t.Fatalf("%s %+v", op.command, res)
					}
					want := c.prefix + newBlock("codex", op.want, c.padding, codexAgent.policy(op.want)) + suffix
					if got := string(read(t, path)); got != want {
						t.Fatalf("migrated to %q\nwant %q", got, want)
					}
					if res = mustApply(t, codexAgent, home, "enable", 0); res.changed || res.threshold != op.want {
						t.Fatalf("migrated block not stable: %+v", res)
					}
					backups, err := filepath.Glob(filepath.Join(home, ".agent-usage-reserve-backup-*"))
					if err != nil || len(backups) != 1 || string(read(t, backups[0])) != legacy {
						t.Fatalf("legacy backup %v %v", backups, err)
					}
				})
			}
			t.Run("disable", func(t *testing.T) {
				home, path := setup(t)
				if res := mustApply(t, codexAgent, home, "disable", 0); res.enabled || !res.changed {
					t.Fatalf("disable %+v", res)
				}
				if got := string(read(t, path)); got != c.prefix+suffix {
					t.Fatalf("disable left %q", got)
				}
			})
		})
	}
}

// Catches a rewrite that recomputes padding from the prefix instead of keeping
// the recorded value, which would make disable remove or leave the wrong bytes.
func TestRewriteKeepsRecordedPadding(t *testing.T) {
	const suffix = "later user text\n"
	for _, a := range agents {
		for _, c := range []struct {
			prefix  string
			padding int
		}{{"user text\n", 0}, {"user text\n\n", 1}, {"user text\n", 2}, {"", 1}, {"", 2}} {
			t.Run(fmt.Sprintf("%s-%q-padding%d", a.name, c.prefix, c.padding), func(t *testing.T) {
				home := t.TempDir()
				path := filepath.Join(home, a.targets[0])
				write(t, path, []byte(c.prefix+newBlock(a.name, 7, c.padding, a.policy(7))+suffix))
				for _, op := range []struct {
					command         string
					threshold, want int
				}{{"set", 20, 20}, {"enable", 15, 15}, {"enable", 0, 15}} {
					res := mustApply(t, a, home, op.command, op.threshold)
					if !res.enabled || res.threshold != op.want || res.changed != (op.threshold != 0) {
						t.Fatalf("%s %d: %+v", op.command, op.threshold, res)
					}
					want := c.prefix + newBlock(a.name, op.want, c.padding, a.policy(op.want)) + suffix
					if got := string(read(t, path)); got != want {
						t.Fatalf("%s %d wrote %q\nwant %q", op.command, op.threshold, got, want)
					}
				}
				mustApply(t, a, home, "disable", 0)
				if got := string(read(t, path)); got != c.prefix+suffix {
					t.Fatalf("disable left %q", got)
				}
			})
		}
	}
}

func TestLegacyMarkerInInactiveFileRefused(t *testing.T) {
	home := t.TempDir()
	regular := filepath.Join(home, "AGENTS.md")
	override := filepath.Join(home, "AGENTS.override.md")
	content := "user text\n" + legacyBlock(10, 1)
	write(t, regular, []byte(content))
	write(t, override, []byte("override"))
	if _, err := apply(codexAgent, home, "enable", 0); err == nil || !strings.Contains(err.Error(), "inactive") {
		t.Fatalf("expected inactive conflict: %v", err)
	}
	if string(read(t, regular)) != content || string(read(t, override)) != "override" {
		t.Fatal("changed files")
	}
}

func TestLegacyAndNewBlocksRefusedAsDuplicate(t *testing.T) {
	current := newBlock("codex", 10, 1, codexAgent.policy(10))
	for name, content := range map[string]string{
		"legacy-then-new":   "user text\n" + legacyBlock(10, 1) + current,
		"new-then-legacy":   "user text\n" + current + legacyBlock(10, 1),
		"two-legacy":        "user text\n" + legacyBlock(10, 1) + legacyBlock(10, 1),
		"legacy-begin-only": "user text\n" + strings.TrimSuffix(legacyBlock(10, 1), legacyEnd) + endLine,
		"new-begin-only":    "user text\n" + strings.TrimSuffix(current, endLine) + legacyEnd,
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "AGENTS.md")
			write(t, path, []byte(content))
			assertRefused(t, codexAgent, home, path, content)
			if strings.HasPrefix(name, "legacy-then") || strings.HasPrefix(name, "new-then") {
				_, err := inspect(codexAgent, home)
				want := "found both an old codex-usage-reserve block and an agent-usage-reserve block in " + filepath.Join(resolved(t, home), "AGENTS.md") + "; an old codex-usage-reserve executable is probably still installed — remove it, then run agent-usage-reserve disable and enable again"
				if err == nil || err.Error() != want {
					t.Fatalf("message %v, want %s", err, want)
				}
			}
		})
	}
}

func TestLegacyMarkerInClaudeFileRefused(t *testing.T) {
	for name, content := range map[string]string{
		"block":     "user text\n" + legacyBlock(10, 1),
		"malformed": "<!-- codex-usage-reserve:managed:v2:begin -->",
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "CLAUDE.md")
			write(t, path, []byte(content))
			assertRefused(t, claudeAgent, home, path, content)
		})
	}
}

func TestInspectDoesNotLockOrCreate(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, a := range agents {
		res := mustInspect(t, a, missing)
		if res.enabled || res.target != filepath.Join(missing, a.targets[0]) {
			t.Fatalf("%s inspect missing home %+v", a.name, res)
		}
		if res = mustApply(t, a, missing, "disable", 0); res.enabled || res.changed {
			t.Fatalf("%s disable missing home %+v", a.name, res)
		}
		if _, err := apply(a, missing, "set", 15); err == nil || !strings.Contains(err.Error(), a.label+" policy is disabled; run enable first") {
			t.Fatalf("%s set on missing home: %v", a.name, err)
		}
	}
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Fatal("read-only operation created home")
	}

	home := t.TempDir()
	mustApply(t, claudeAgent, home, "enable", 12)
	lock := filepath.Join(home, ".agent-usage-reserve.lock")
	write(t, lock, []byte("owner"))
	if res := mustInspect(t, claudeAgent, home); !res.enabled || res.threshold != 12 {
		t.Fatalf("inspect with foreign lock %+v", res)
	}
	if _, err := apply(claudeAgent, home, "set", 20); err == nil {
		t.Fatal("ignored lock")
	}
	if string(read(t, lock)) != "owner" {
		t.Fatal("deleted another lock")
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	mustInspect(t, claudeAgent, home)
	if _, err := os.Lstat(lock); !os.IsNotExist(err) {
		t.Fatal("inspect left a lock")
	}
}

func TestHomeValidationUsesAgentLabel(t *testing.T) {
	for _, a := range agents {
		t.Run(a.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "file")
			write(t, file, []byte("not a directory"))
			if _, err := apply(a, file, "enable", 0); err == nil || !strings.Contains(err.Error(), a.label+" home is not a directory") {
				t.Fatalf("enable on file home: %v", err)
			}
			if _, err := inspect(a, file); err == nil || !strings.Contains(err.Error(), a.label+" home is not a directory") {
				t.Fatalf("inspect on file home: %v", err)
			}
			home := t.TempDir()
			alias := filepath.Join(t.TempDir(), "alias")
			if err := os.Symlink(home, alias); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if _, err := apply(a, alias, "enable", 0); err == nil || !strings.Contains(err.Error(), "symlink "+a.label+" home") {
				t.Fatalf("enable on symlink home: %v", err)
			}
			if _, err := inspect(a, alias); err == nil || !strings.Contains(err.Error(), "symlink "+a.label+" home") {
				t.Fatalf("inspect on symlink home: %v", err)
			}
			if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
				t.Fatalf("wrote through home symlink: %v %v", entries, err)
			}
		})
	}
}

func TestClaudeBackupAndMode(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "CLAUDE.md")
	write(t, path, []byte("original"))
	mustApply(t, claudeAgent, home, "enable", 0)
	backups, err := filepath.Glob(filepath.Join(home, ".agent-usage-reserve-backup-*"))
	if err != nil || len(backups) != 1 || string(read(t, backups[0])) != "original" {
		t.Fatalf("backups %v %v", backups, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("mode %v %v", info, err)
	}
	info, err = os.Stat(backups[0])
	if err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatalf("backup is not private: %v %v", info, err)
	}
}

func TestApplyRejectsInvalidInput(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "CLAUDE.md")
	write(t, path, []byte("original"))
	for _, c := range []struct {
		command   string
		threshold int
	}{{"status", 0}, {"bogus", 0}, {"enable", 100}, {"enable", -1}, {"set", 100}} {
		if _, err := apply(claudeAgent, home, c.command, c.threshold); err == nil {
			t.Fatalf("accepted %s %d", c.command, c.threshold)
		}
	}
	if string(read(t, path)) != "original" {
		t.Fatal("changed file on invalid input")
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := apply(claudeAgent, missing, "enable", 100); err == nil {
		t.Fatal("accepted invalid threshold")
	}
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Fatal("invalid input created home")
	}
}

// Agents keep separate files, so sharing one directory does not mix their blocks.
func TestAgentsShareDirectoryIndependently(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, "AGENTS.md"), []byte("codex text\n"))
	write(t, filepath.Join(home, "CLAUDE.md"), []byte("claude text"))
	mustApply(t, codexAgent, home, "enable", 11)
	mustApply(t, claudeAgent, home, "enable", 22)
	if res := mustInspect(t, codexAgent, home); !res.enabled || res.threshold != 11 {
		t.Fatalf("codex %+v", res)
	}
	if res := mustInspect(t, claudeAgent, home); !res.enabled || res.threshold != 22 {
		t.Fatalf("claude %+v", res)
	}
	mustApply(t, codexAgent, home, "disable", 0)
	mustApply(t, claudeAgent, home, "disable", 0)
	if string(read(t, filepath.Join(home, "AGENTS.md"))) != "codex text\n" || string(read(t, filepath.Join(home, "CLAUDE.md"))) != "claude text" {
		t.Fatal("shared directory roundtrip changed bytes")
	}
}
