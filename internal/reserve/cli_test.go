package reserve

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	claudeNote = "  (works in the Claude desktop app's Code tab)"
	policyOnly = "Installed policy only; account usage and compliance are not checked.\n"
	choiceMenu = "Found Codex and Claude. Apply the usage reserve to:\n  1) Both\n  2) Claude only\n  3) Codex only\nChoose [1]:"
	notFound   = "No Codex or Claude home found.\n"
)

// TestMain points every home lookup at a sandbox before any test runs, so a
// test that forgets its own setup still cannot reach the real ~/.claude or ~/.codex.
func TestMain(m *testing.M) {
	sandbox, err := os.MkdirTemp("", "agent-usage-reserve-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, name := range []string{"HOME", "USERPROFILE", "CODEX_HOME", "CLAUDE_CONFIG_DIR"} {
		if err = os.Setenv(name, filepath.Join(sandbox, "unset-"+name)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	code := m.Run()
	os.RemoveAll(sandbox)
	os.Exit(code)
}

// cliHomes points HOME, CLAUDE_CONFIG_DIR and CODEX_HOME into one temp tree
// per test, with only the requested agent homes present.
type cliHomes struct {
	root, user, claude, codex string
}

func newHomes(t *testing.T, claude, codex bool) cliHomes {
	t.Helper()
	root := t.TempDir()
	h := cliHomes{
		root:   root,
		user:   filepath.Join(root, "user"),
		claude: filepath.Join(root, "claude"),
		codex:  filepath.Join(root, "codex"),
	}
	if err := os.Mkdir(h.user, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", h.user)
	t.Setenv("USERPROFILE", h.user) // os.UserHomeDir on Windows
	t.Setenv("CLAUDE_CONFIG_DIR", h.claude)
	t.Setenv("CODEX_HOME", h.codex)
	for dir, present := range map[string]bool{h.claude: claude, h.codex: codex} {
		if present {
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	return h
}

// Effective target paths as printed, for homes that exist.
func (h cliHomes) claudeFile(t *testing.T) string {
	return filepath.Join(resolved(t, h.claude), "CLAUDE.md")
}

func (h cliHomes) codexFile(t *testing.T) string {
	return filepath.Join(resolved(t, h.codex), "AGENTS.md")
}

func cli(t *testing.T, input string, interactive bool, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := Run(args, strings.NewReader(input), &out, interactive, "test")
	return out.String(), err
}

func mustCLI(t *testing.T, args ...string) string {
	t.Helper()
	out, err := cli(t, "", false, args...)
	if err != nil {
		t.Fatalf("%v: %v (output %q)", args, err, out)
	}
	return out
}

func claudeOn(threshold int, path string) string {
	return fmt.Sprintf("✓ Claude  %d%%  %s%s\n", threshold, path, claudeNote)
}

func codexOn(threshold int, path string) string {
	return fmt.Sprintf("✓ Codex   %d%%  %s\n", threshold, path)
}

func claudeOff(path string) string { return "– Claude  disabled  " + path + claudeNote + "\n" }
func codexOff(path string) string  { return "– Codex   disabled  " + path + "\n" }

// tree records every directory and file (with contents) under root, so tests
// can assert that nothing at all was created, changed, locked or backed up.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			files[p+string(filepath.Separator)] = ""
			return nil
		}
		b, err := os.ReadFile(p)
		files[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertTree(t *testing.T, root string, want map[string]string) {
	t.Helper()
	if got := tree(t, root); !maps.Equal(got, want) {
		t.Fatalf("files changed under %s:\n%v\nwant\n%v", root, keys(got), keys(want))
	}
}

func keys(m map[string]string) []string {
	return slices.Sorted(maps.Keys(m))
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists: %v", path, err)
	}
}

func TestEnableWithoutAgentChangesNothing(t *testing.T) {
	h := newHomes(t, false, false)
	before := tree(t, h.root)
	for _, c := range []struct {
		interactive bool
		args        []string
	}{{false, []string{"enable"}}, {true, []string{"enable"}}, {false, []string{"enable", "--threshold", "15"}}} {
		out, err := cli(t, "1\n", c.interactive, c.args...)
		if !errors.Is(err, ErrNoAgent) || errors.Is(err, ErrNeedsChoice) || out != "" {
			t.Fatalf("%v interactive=%v: %q %v", c.args, c.interactive, out, err)
		}
		if msg := err.Error(); !strings.Contains(msg, "Codex or Claude was not found") || !strings.Contains(msg, "enable again") {
			t.Fatalf("guidance %q", msg)
		}
	}
	assertTree(t, h.root, before)
}

func TestEnableSingleFoundAgentWithoutPrompt(t *testing.T) {
	t.Run("claude", func(t *testing.T) {
		h := newHomes(t, true, false)
		// Not interactive: any prompt would fail the command.
		if out := mustCLI(t, "enable"); out != claudeOn(10, h.claudeFile(t)) {
			t.Fatalf("output %q", out)
		}
		if res := mustInspect(t, claudeAgent, h.claude); !res.enabled || res.threshold != 10 {
			t.Fatalf("claude %+v", res)
		}
		assertMissing(t, h.codex)
	})
	t.Run("codex", func(t *testing.T) {
		h := newHomes(t, false, true)
		if out := mustCLI(t, "enable", "--threshold", "15"); out != codexOn(15, h.codexFile(t)) {
			t.Fatalf("output %q", out)
		}
		if res := mustInspect(t, codexAgent, h.codex); !res.enabled || res.threshold != 15 {
			t.Fatalf("codex %+v", res)
		}
		assertMissing(t, h.claude)
	})
}

func TestEnablePromptChoices(t *testing.T) {
	for _, c := range []struct {
		input         string
		claude, codex bool
		asked         int
	}{
		{"\n", true, true, 1},
		{"1\n", true, true, 1},
		{"2\n", true, false, 1},
		{"3\n", false, true, 1},
		{"x\n2\n", true, false, 2},
		{" 3 \r\n", false, true, 1},
		{"0\n4\n1\n", true, true, 3},
		{"x\n\n", true, true, 2},
		{"2", true, false, 1}, // final answer without a newline
	} {
		t.Run(fmt.Sprintf("%q", c.input), func(t *testing.T) {
			h := newHomes(t, true, true)
			out, err := cli(t, c.input, true, "enable")
			if err != nil {
				t.Fatalf("%v (output %q)", err, out)
			}
			if !strings.HasPrefix(out, choiceMenu) || strings.Count(out, "Choose [1]:") != c.asked || strings.Count(out, "Found Codex and Claude") != 1 {
				t.Fatalf("prompt %q", out)
			}
			want := ""
			if c.claude {
				want += claudeOn(10, h.claudeFile(t))
			} else {
				assertMissing(t, filepath.Join(h.claude, "CLAUDE.md"))
			}
			if c.codex {
				want += codexOn(10, h.codexFile(t))
			} else {
				assertMissing(t, filepath.Join(h.codex, "AGENTS.md"))
			}
			if !strings.HasSuffix(out, "Choose [1]: "+want) {
				t.Fatalf("output %q, want suffix %q", out, want)
			}
			if mustInspect(t, claudeAgent, h.claude).enabled != c.claude || mustInspect(t, codexAgent, h.codex).enabled != c.codex {
				t.Fatal("wrong agents enabled")
			}
		})
	}
}

func TestEnablePromptThreeInvalidAnswersNeedChoice(t *testing.T) {
	for _, input := range []string{"x\nx\nx\n", "x\nx\nx\n1\n", "4\nboth\n-1\n2\n"} {
		t.Run(fmt.Sprintf("%q", input), func(t *testing.T) {
			h := newHomes(t, true, true)
			before := tree(t, h.root)
			out, err := cli(t, input, true, "enable")
			if !errors.Is(err, ErrNeedsChoice) || errors.Is(err, ErrNoAgent) {
				t.Fatalf("expected ErrNeedsChoice: %v", err)
			}
			for _, want := range []string{"nothing was changed", "agent-usage-reserve enable --only all", "agent-usage-reserve enable --only claude", "agent-usage-reserve enable --only codex"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("guidance lacks %q: %v", want, err)
				}
			}
			if strings.Count(out, "Choose [1]:") != 3 {
				t.Fatalf("%q", out)
			}
			assertTree(t, h.root, before)
		})
	}
	// The same for the yes/no question about a newly found agent.
	h := newHomes(t, true, true)
	mustCLI(t, "enable", "--only", "codex")
	before := tree(t, h.root)
	out, err := cli(t, "maybe\nx\n?\n", true, "enable")
	if !errors.Is(err, ErrNeedsChoice) || !strings.Contains(err.Error(), "agent-usage-reserve enable --only all") || strings.Count(out, "[Y/n]:") != 3 {
		t.Fatalf("%q %v", out, err)
	}
	assertTree(t, h.root, before)
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("input/output error") }

// Any read failure, not only end of input, means there was no way to ask.
func TestEnablePromptReadErrorNeedsChoice(t *testing.T) {
	h := newHomes(t, true, true)
	before := tree(t, h.root)
	var out bytes.Buffer
	err := Run([]string{"enable"}, failingReader{}, &out, true, "test")
	if !errors.Is(err, ErrNeedsChoice) || !strings.Contains(err.Error(), "agent-usage-reserve enable --only all") || !strings.Contains(err.Error(), "input/output error") {
		t.Fatalf("%q %v", out.String(), err)
	}
	assertTree(t, h.root, before)
}

// End of input before a valid answer (Ctrl-D, or a character device such as
// /dev/null that is not a terminal) is treated as having no way to ask.
func TestEnablePromptEndOfInputNeedsChoice(t *testing.T) {
	for _, input := range []string{"", "x\n"} {
		h := newHomes(t, true, true)
		before := tree(t, h.root)
		out, err := cli(t, input, true, "enable")
		if !errors.Is(err, ErrNeedsChoice) || !strings.Contains(err.Error(), "agent-usage-reserve enable --only all") {
			t.Fatalf("%q: %v", input, err)
		}
		if !strings.HasPrefix(out, choiceMenu) {
			t.Fatalf("prompt %q", out)
		}
		assertTree(t, h.root, before)
	}
}

func TestEnableWithoutTerminalPrintsOnlyCommands(t *testing.T) {
	h := newHomes(t, true, true)
	before := tree(t, h.root)
	out, err := cli(t, "1\n", false, "enable")
	if !errors.Is(err, ErrNeedsChoice) || errors.Is(err, ErrNoAgent) || out != "" {
		t.Fatalf("%q %v", out, err)
	}
	for _, command := range []string{"agent-usage-reserve enable --only all\n", "agent-usage-reserve enable --only claude\n", "agent-usage-reserve enable --only codex"} {
		if !strings.Contains(err.Error(), command) {
			t.Fatalf("guidance lacks %q: %v", command, err)
		}
	}
	_, err = cli(t, "", false, "enable", "--threshold", "15")
	if !errors.Is(err, ErrNeedsChoice) || !strings.Contains(err.Error(), "agent-usage-reserve enable --only claude --threshold 15") {
		t.Fatalf("guidance drops threshold: %v", err)
	}
	assertTree(t, h.root, before)
}

func TestEnableAsksAboutNewlyFoundAgent(t *testing.T) {
	for _, existing := range agents {
		var fresh agent
		for _, a := range agents {
			if a.name != existing.name {
				fresh = a
			}
		}
		for _, c := range []struct {
			input  string
			enable bool
			asked  int
		}{{"\n", true, 1}, {"y\n", true, 1}, {"YES\n", true, 1}, {"n\n", false, 1}, {"No\n", false, 1}, {"maybe\nn\n", false, 2}} {
			t.Run(fmt.Sprintf("%s-enabled-%q", existing.name, c.input), func(t *testing.T) {
				h := newHomes(t, true, true)
				homes := map[string]string{"claude": h.claude, "codex": h.codex}
				mustCLI(t, "enable", "--only", existing.name, "--threshold", "12")
				existingBefore := tree(t, homes[existing.name])
				out, err := cli(t, c.input, true, "enable")
				if err != nil {
					t.Fatalf("%v (output %q)", err, out)
				}
				question := "Also enable for " + fresh.label + "? [Y/n]:"
				if !strings.HasPrefix(out, question) || strings.Count(out, question) != c.asked || strings.Contains(out, "Choose [1]") {
					t.Fatalf("prompt %q", out)
				}
				assertTree(t, homes[existing.name], existingBefore)
				if res := mustInspect(t, fresh, homes[fresh.name]); res.enabled != c.enable || (c.enable && res.threshold != 10) {
					t.Fatalf("%s %+v", fresh.name, res)
				}
				line := func(a agent, threshold int) string {
					if a.name == "claude" {
						return claudeOn(threshold, h.claudeFile(t))
					}
					return codexOn(threshold, h.codexFile(t))
				}
				want := ""
				for _, a := range agents {
					switch {
					case a.name == existing.name:
						want += line(a, 12)
					case c.enable:
						want += line(a, 10)
					}
				}
				if !strings.HasSuffix(out, "[Y/n]: "+want) {
					t.Fatalf("output %q, want suffix %q", out, want)
				}
				if !c.enable {
					assertMissing(t, filepath.Join(homes[fresh.name], fresh.targets[0]))
				}
			})
		}
		t.Run(existing.name+"-enabled-no-terminal", func(t *testing.T) {
			h := newHomes(t, true, true)
			mustCLI(t, "enable", "--only", existing.name)
			before := tree(t, h.root)
			out, err := cli(t, "y\n", false, "enable")
			if !errors.Is(err, ErrNeedsChoice) || out != "" {
				t.Fatalf("%q %v", out, err)
			}
			for _, command := range []string{"agent-usage-reserve enable --only all", "agent-usage-reserve enable --only " + existing.name} {
				if !strings.Contains(err.Error(), command) {
					t.Fatalf("guidance lacks %q: %v", command, err)
				}
			}
			if strings.Contains(err.Error(), "--only "+fresh.name) {
				t.Fatalf("guidance offers dropping %s: %v", existing.name, err)
			}
			assertTree(t, h.root, before)
		})
	}
}

func TestRepeatEnableKeepsThresholdsWithoutPrompt(t *testing.T) {
	h := newHomes(t, true, true)
	mustCLI(t, "enable", "--only", "claude", "--threshold", "12")
	mustCLI(t, "enable", "--only", "codex", "--threshold", "34")
	before := tree(t, h.root)
	if out := mustCLI(t, "enable"); out != claudeOn(12, h.claudeFile(t))+codexOn(34, h.codexFile(t)) {
		t.Fatalf("output %q", out)
	}
	assertTree(t, h.root, before)
	if out := mustCLI(t, "enable", "--threshold", "20"); out != claudeOn(20, h.claudeFile(t))+codexOn(20, h.codexFile(t)) {
		t.Fatalf("output %q", out)
	}
}

// An installed codex-usage-reserve block counts as enabled and is migrated.
func TestEnableMigratesLegacyCodexBlock(t *testing.T) {
	h := newHomes(t, false, true)
	write(t, filepath.Join(h.codex, "AGENTS.md"), []byte("user text\n"+legacyBlock(7, 1)))
	if out := mustCLI(t, "status"); out != codexOn(7, h.codexFile(t))+policyOnly {
		t.Fatalf("status %q", out)
	}
	if out := mustCLI(t, "enable"); out != codexOn(7, h.codexFile(t)) {
		t.Fatalf("output %q", out)
	}
	if got := string(read(t, filepath.Join(h.codex, "AGENTS.md"))); got != "user text\n"+newBlock("codex", 7, 1, codexAgent.policy(7)) {
		t.Fatalf("migrated %q", got)
	}
}

func TestSelectedHomesAreCreatedByEnable(t *testing.T) {
	t.Run("only", func(t *testing.T) {
		h := newHomes(t, false, false)
		if out := mustCLI(t, "enable", "--only", "claude"); out != claudeOn(10, h.claudeFile(t)) {
			t.Fatalf("output %q", out)
		}
		assertMissing(t, h.codex)
	})
	t.Run("only-all", func(t *testing.T) {
		h := newHomes(t, false, false)
		if out := mustCLI(t, "enable", "--only", "all"); out != claudeOn(10, h.claudeFile(t))+codexOn(10, h.codexFile(t)) {
			t.Fatalf("output %q", out)
		}
	})
	t.Run("home-flag-selects", func(t *testing.T) {
		h := newHomes(t, false, true)
		explicit := filepath.Join(h.root, "explicit", "claude")
		out := mustCLI(t, "enable", "--claude-home", explicit)
		if out != claudeOn(10, filepath.Join(resolved(t, explicit), "CLAUDE.md")) {
			t.Fatalf("output %q", out)
		}
		assertMissing(t, filepath.Join(h.codex, "AGENTS.md"))
		assertMissing(t, h.claude)
	})
	t.Run("both-home-flags", func(t *testing.T) {
		h := newHomes(t, true, true)
		claude, codex := filepath.Join(h.root, "c1"), filepath.Join(h.root, "c2")
		out := mustCLI(t, "enable", "--codex-home", codex, "--claude-home", claude)
		if out != claudeOn(10, filepath.Join(resolved(t, claude), "CLAUDE.md"))+codexOn(10, filepath.Join(resolved(t, codex), "AGENTS.md")) {
			t.Fatalf("output %q", out)
		}
		assertMissing(t, filepath.Join(h.claude, "CLAUDE.md"))
		assertMissing(t, filepath.Join(h.codex, "AGENTS.md"))
	})
	t.Run("only-consistent-with-home-flag", func(t *testing.T) {
		h := newHomes(t, false, false)
		codex := filepath.Join(h.root, "c2")
		if out := mustCLI(t, "enable", "--only", "codex", "--codex-home", codex); out != codexOn(10, filepath.Join(resolved(t, codex), "AGENTS.md")) {
			t.Fatalf("output %q", out)
		}
		assertMissing(t, h.claude)
	})
}

func TestInvalidSelectionAndFlagsRejected(t *testing.T) {
	h := newHomes(t, true, true)
	before := tree(t, h.root)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"enable", "--only", "bogus"}, `--only must be claude, codex or all, not "bogus"`},
		{[]string{"enable", "--only", ""}, `--only must be claude, codex or all, not ""`},
		{[]string{"enable", "--only", "Claude"}, `--only must be claude, codex or all, not "Claude"`},
		{[]string{"enable", "--only", "claude,codex"}, `--only must be claude, codex or all, not "claude,codex"`},
		{[]string{"enable", "--only"}, "flag needs an argument: -only"},
		{[]string{"set", "15", "--only", "both"}, `--only must be claude, codex or all, not "both"`},
		{[]string{"status", "--only", "none"}, `--only must be claude, codex or all, not "none"`},
		{[]string{"disable", "--only", "x"}, `--only must be claude, codex or all, not "x"`},
		{[]string{"enable", "--only", "claude", "--codex-home", h.codex}, "--codex-home conflicts with --only claude"},
		{[]string{"disable", "--only", "codex", "--claude-home", h.claude}, "--claude-home conflicts with --only codex"},
		{[]string{"enable", "--codex-home", ""}, "--codex-home requires a path"},
		{[]string{"enable", "--threshold", "0"}, "from 1 to 99"},
		{[]string{"set", "15", "--threshold", "20"}, "flag provided but not defined: -threshold"},
		{[]string{"status", "--threshold", "20"}, "flag provided but not defined: -threshold"},
		{[]string{"status", "extra"}, `unexpected argument "extra"`},
		{[]string{"bogus"}, `unknown command "bogus"`},
	} {
		out, err := cli(t, "1\n", true, c.args...)
		if err == nil || errors.Is(err, ErrNeedsChoice) || errors.Is(err, ErrNoAgent) || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%v: want error containing %q, got %q %v", c.args, c.want, out, err)
		}
	}
	assertTree(t, h.root, before)
}

func TestSetActsOnEnabledOrSelectedAgents(t *testing.T) {
	h := newHomes(t, true, true)
	mustCLI(t, "enable", "--only", "all")
	if out := mustCLI(t, "set", "15"); out != claudeOn(15, h.claudeFile(t))+codexOn(15, h.codexFile(t)) {
		t.Fatalf("set all %q", out)
	}
	codexBefore := tree(t, h.codex)
	if out := mustCLI(t, "set", "25", "--only", "claude"); out != claudeOn(25, h.claudeFile(t)) {
		t.Fatalf("set claude %q", out)
	}
	assertTree(t, h.codex, codexBefore)
	mustCLI(t, "disable", "--only", "codex")
	before := tree(t, h.root)
	for _, args := range [][]string{{"set", "30", "--only", "codex"}, {"set", "30", "--only", "all"}, {"set", "30", "--codex-home", h.codex}} {
		out, err := cli(t, "", false, args...)
		if err == nil || !strings.Contains(err.Error(), "Codex policy is disabled; run enable first") || out != "" {
			t.Fatalf("%v: %q %v", args, out, err)
		}
		assertTree(t, h.root, before)
	}
	if out := mustCLI(t, "set", "30"); out != claudeOn(30, h.claudeFile(t)) {
		t.Fatalf("set enabled %q", out)
	}
	if res := mustInspect(t, codexAgent, h.codex); res.enabled {
		t.Fatalf("set enabled codex %+v", res)
	}
	mustCLI(t, "disable")
	assertSetRefused := func(root string) {
		t.Helper()
		before := tree(t, root)
		if _, err := cli(t, "", false, "set", "15"); err == nil || !strings.Contains(err.Error(), "run enable first") {
			t.Fatalf("set with nothing enabled: %v", err)
		}
		assertTree(t, root, before)
	}
	assertSetRefused(h.root)
	assertSetRefused(newHomes(t, false, false).root)
	missing := newHomes(t, false, false)
	if _, err := cli(t, "", false, "set", "15", "--only", "claude"); err == nil || !strings.Contains(err.Error(), "Claude policy is disabled; run enable first") {
		t.Fatalf("set on missing selected home: %v", err)
	}
	assertMissing(t, missing.claude)
}

func TestDisableAllIsIdempotent(t *testing.T) {
	h := newHomes(t, true, true)
	claudePath, codexPath := filepath.Join(h.claude, "CLAUDE.md"), filepath.Join(h.codex, "AGENTS.md")
	write(t, claudePath, []byte("claude text\n"))
	write(t, codexPath, []byte("codex text"))
	mustCLI(t, "enable", "--only", "all", "--threshold", "15")
	want := claudeOff(h.claudeFile(t)) + codexOff(h.codexFile(t))
	if out := mustCLI(t, "disable"); out != want {
		t.Fatalf("disable %q", out)
	}
	if string(read(t, claudePath)) != "claude text\n" || string(read(t, codexPath)) != "codex text" {
		t.Fatal("disable did not restore bytes")
	}
	after := tree(t, h.root)
	if out := mustCLI(t, "disable"); out != want {
		t.Fatalf("repeat disable %q", out)
	}
	assertTree(t, h.root, after)

	mustCLI(t, "enable", "--only", "codex")
	if out := mustCLI(t, "disable"); out != want {
		t.Fatalf("disable with one enabled %q", out)
	}

	missing := filepath.Join(h.root, "missing")
	if out := mustCLI(t, "disable", "--codex-home", missing); out != codexOff(filepath.Join(missing, "AGENTS.md")) {
		t.Fatalf("disable missing home %q", out)
	}
	assertMissing(t, missing)

	none := newHomes(t, false, false)
	before := tree(t, none.root)
	if out := mustCLI(t, "disable"); out != notFound {
		t.Fatalf("disable with nothing found %q", out)
	}
	assertTree(t, none.root, before)
}

func TestStatusLines(t *testing.T) {
	h := newHomes(t, true, true)
	mustCLI(t, "enable", "--only", "all")
	claudePath, codexPath := h.claudeFile(t), h.codexFile(t)
	before := tree(t, h.root)
	want := "✓ Claude  10%  " + claudePath + "  (works in the Claude desktop app's Code tab)\n" +
		"✓ Codex   10%  " + codexPath + "\n" +
		"Installed policy only; account usage and compliance are not checked.\n"
	if out := mustCLI(t, "status"); out != want {
		t.Fatalf("status %q\nwant %q", out, want)
	}
	assertTree(t, h.root, before)

	mustCLI(t, "disable", "--only", "codex")
	want = "✓ Claude  10%  " + claudePath + "  (works in the Claude desktop app's Code tab)\n" +
		"– Codex   disabled  " + codexPath + "\n" + policyOnly
	if out := mustCLI(t, "status"); out != want {
		t.Fatalf("status %q\nwant %q", out, want)
	}
	if out := mustCLI(t, "status", "--only", "codex"); out != codexOff(codexPath)+policyOnly {
		t.Fatalf("status --only codex %q", out)
	}

	mustCLI(t, "disable")
	if out := mustCLI(t, "status"); out != claudeOff(claudePath)+codexOff(codexPath)+policyOnly {
		t.Fatalf("status all disabled %q", out)
	}

	only := newHomes(t, false, true)
	if out := mustCLI(t, "status"); out != codexOff(only.codexFile(t))+policyOnly {
		t.Fatalf("status with codex only %q", out)
	}

	none := newHomes(t, false, false)
	before = tree(t, none.root)
	if out := mustCLI(t, "status"); out != notFound {
		t.Fatalf("status with nothing found %q", out)
	}
	missing := filepath.Join(none.root, "missing")
	if out := mustCLI(t, "status", "--claude-home", missing); out != claudeOff(filepath.Join(missing, "CLAUDE.md"))+policyOnly {
		t.Fatalf("status of selected missing home %q", out)
	}
	assertTree(t, none.root, before)
}

// Lines for agents already processed are printed before the first error.
func TestFirstErrorStopsAfterPrintedLines(t *testing.T) {
	h := newHomes(t, true, false)
	write(t, h.codex, []byte("not a directory"))
	before := tree(t, h.root)
	// A regular file at a home is not a detected agent: the command acts on
	// Claude alone, without a prompt, and never touches the file.
	if out, err := cli(t, "", false, "status"); err != nil || out != claudeOff(h.claudeFile(t))+policyOnly {
		t.Fatalf("status: %q %v", out, err)
	}
	assertTree(t, h.root, before)
	if out, err := cli(t, "", false, "enable"); err != nil || out != claudeOn(10, h.claudeFile(t)) {
		t.Fatalf("enable: %q %v", out, err)
	}
	if got := string(read(t, h.codex)); got != "not a directory" {
		t.Fatalf("codex file changed: %q", got)
	}
	// An explicit selection still reaches the engine, which refuses the file.
	out, err := cli(t, "", false, "enable", "--only", "all")
	if err == nil || !strings.Contains(err.Error(), "Codex home is not a directory") || out != claudeOn(10, h.claudeFile(t)) {
		t.Fatalf("enable --only all: %q %v", out, err)
	}
	out, err = cli(t, "", false, "status", "--only", "all")
	if err == nil || !strings.Contains(err.Error(), "Codex home is not a directory") || out != claudeOn(10, h.claudeFile(t)) {
		t.Fatalf("status --only all: %q %v", out, err)
	}
}

// A found home that never had the policy is reported but not touched, so a
// foreign lock there cannot block disabling the other agent.
func TestDisableSkipsFoundAgentsThatAreNotEnabled(t *testing.T) {
	h := newHomes(t, true, true)
	mustCLI(t, "enable", "--only", "codex")
	lock := filepath.Join(h.claude, ".agent-usage-reserve.lock")
	write(t, lock, []byte("owner"))
	if out := mustCLI(t, "disable"); out != claudeOff(h.claudeFile(t))+codexOff(h.codexFile(t)) {
		t.Fatalf("disable %q", out)
	}
	if res := mustInspect(t, codexAgent, h.codex); res.enabled {
		t.Fatalf("codex still enabled %+v", res)
	}
	if string(read(t, lock)) != "owner" {
		t.Fatal("touched the claude lock")
	}
	// Selecting the agent applies the engine, which honors the lock.
	if _, err := cli(t, "", false, "disable", "--only", "claude"); err == nil || !strings.Contains(err.Error(), "cannot acquire lock") {
		t.Fatalf("disable --only claude: %v", err)
	}
}

func TestHomeResolution(t *testing.T) {
	h := newHomes(t, false, false)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	defaultClaude := filepath.Join(h.user, ".claude")
	if err := os.Mkdir(defaultClaude, 0700); err != nil {
		t.Fatal(err)
	}
	if out := mustCLI(t, "enable"); out != claudeOn(10, filepath.Join(resolved(t, defaultClaude), "CLAUDE.md")) {
		t.Fatalf("default home %q", out)
	}
	assertMissing(t, filepath.Join(h.user, ".codex"))

	// A nonempty environment variable wins over the default directory.
	t.Setenv("CLAUDE_CONFIG_DIR", h.claude)
	if out := mustCLI(t, "status"); out != notFound {
		t.Fatalf("env home %q", out)
	}

	// A relative flag is made absolute.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(h.root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if out := mustCLI(t, "status", "--codex-home", "rel"); out != codexOff(filepath.Join(cwd, "rel", "AGENTS.md"))+policyOnly {
		t.Fatalf("relative home %q", out)
	}
}

func TestHelpAndVersion(t *testing.T) {
	h := newHomes(t, true, true)
	before := tree(t, h.root)
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"help"}, {"enable", "--help"}, {"set", "--help"}, {"set", "15", "--help"}, {"status", "-h"}, {"disable", "--help"}, {"enable", "--only", "claude", "--help"}} {
		out := mustCLI(t, args...)
		for _, want := range []string{
			"agent-usage-reserve enable  [--threshold 10] [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]",
			"agent-usage-reserve set 15  [--only claude|codex|all]",
			"agent-usage-reserve status",
			"agent-usage-reserve disable [--only claude|codex|all]",
			"agent-usage-reserve --version",
			"CLAUDE_CONFIG_DIR", "~/.claude", "CODEX_HOME", "~/.codex",
			"Code tab",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("%v help lacks %q:\n%s", args, want, out)
			}
		}
	}
	if out := mustCLI(t, "--version"); out != "test\n" {
		t.Fatalf("version %q", out)
	}
	assertTree(t, h.root, before)
}

// A symlinked CLAUDE.md makes Claude fail while Codex stays healthy.
func brokenClaudeHomes(t *testing.T) (cliHomes, string) {
	t.Helper()
	h := newHomes(t, true, true)
	target := filepath.Join(h.root, "real-claude.md")
	write(t, target, []byte("private\n"))
	if err := os.Symlink(target, filepath.Join(h.claude, "CLAUDE.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	write(t, filepath.Join(h.codex, "AGENTS.md"), []byte(codexUserText))
	mustCLI(t, "enable", "--only", "codex")
	return h, target
}

const codexUserText = "user text\n"

func TestStatusIsolatesFailingAgent(t *testing.T) {
	h, target := brokenClaudeHomes(t)
	before := tree(t, h.root)
	out, err := cli(t, "", false, "status")
	want := "! Claude  error: refusing symlink instruction target: " + filepath.Join(resolved(t, h.claude), "CLAUDE.md") + "\n" + codexOn(10, h.codexFile(t)) + policyOnly
	if err == nil || errors.Is(err, ErrNeedsChoice) || errors.Is(err, ErrNoAgent) || out != want {
		t.Fatalf("%q %v", out, err)
	}
	assertTree(t, h.root, before)
	if string(read(t, target)) != "private\n" {
		t.Fatal("touched the symlink target")
	}
	// With a selection the failure stays fail-fast and unwrapped.
	out, err = cli(t, "", false, "status", "--only", "claude")
	if err == nil || out != "" || !strings.Contains(err.Error(), "refusing symlink instruction target") {
		t.Fatalf("%q %v", out, err)
	}
}

func TestDisableIsolatesFailingAgent(t *testing.T) {
	h, target := brokenClaudeHomes(t)
	claudeBefore := tree(t, h.claude)
	out, err := cli(t, "", false, "disable")
	want := "! Claude  error: refusing symlink instruction target: " + filepath.Join(resolved(t, h.claude), "CLAUDE.md") + "\n" + codexOff(h.codexFile(t))
	if err == nil || errors.Is(err, ErrNeedsChoice) || out != want {
		t.Fatalf("%q %v", out, err)
	}
	if got := string(read(t, filepath.Join(h.codex, "AGENTS.md"))); got != codexUserText {
		t.Fatalf("codex file not restored byte-exactly: %q", got)
	}
	assertTree(t, h.claude, claudeBefore)
	if string(read(t, target)) != "private\n" {
		t.Fatal("touched the symlink target")
	}
}

func TestEnableAndSetNameFailingAgent(t *testing.T) {
	h, _ := brokenClaudeHomes(t)
	claudeBefore := tree(t, h.claude)
	codexBefore := tree(t, h.codex)
	path := filepath.Join(resolved(t, h.claude), "CLAUDE.md")
	for _, c := range []struct {
		args []string
		hint string
	}{
		{[]string{"enable"}, "to leave Claude alone, run: agent-usage-reserve enable --only codex"},
		{[]string{"set", "15"}, "to leave Claude alone, run: agent-usage-reserve set 15 --only codex"},
	} {
		out, err := cli(t, "", false, c.args...)
		if err == nil || errors.Is(err, ErrNeedsChoice) || out != "" {
			t.Fatalf("%v: %q %v", c.args, out, err)
		}
		if want := "Claude: refusing symlink instruction target: " + path + "; " + c.hint; err.Error() != want {
			t.Fatalf("%v: %v, want %s", c.args, err, want)
		}
		assertTree(t, h.claude, claudeBefore)
		assertTree(t, h.codex, codexBefore)
	}
	// With no other agent found there is no hint.
	if err := os.RemoveAll(h.codex); err != nil {
		t.Fatal(err)
	}
	_, err := cli(t, "", false, "enable")
	if err == nil || err.Error() != "Claude: refusing symlink instruction target: "+path {
		t.Fatalf("%v", err)
	}
}
