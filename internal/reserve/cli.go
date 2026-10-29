package reserve

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// ErrNeedsChoice means enable had to ask which agents to use but could not:
// stdin is not a terminal, or no valid answer came (input ended or failed, or
// three answers were invalid). Nothing was changed, and the wrapping error lists the exact --only commands to run.
var ErrNeedsChoice = errors.New("a choice is needed")

// ErrNoAgent means enable found neither agent home and none was selected.
// Nothing was changed.
var ErrNoAgent = errors.New("Codex or Claude was not found")

const help = `Usage:
  agent-usage-reserve enable  [--threshold 10] [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
  agent-usage-reserve set 15  [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
  agent-usage-reserve status  [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
  agent-usage-reserve disable [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
  agent-usage-reserve --version

Homes: --claude-home, then CLAUDE_CONFIG_DIR, then ~/.claude (CLAUDE.md);
       --codex-home, then CODEX_HOME, then ~/.codex (AGENTS.md, or
       AGENTS.override.md when it is nonempty).
An agent is found when its home exists. --only or a home flag selects agents;
enable creates a selected home that is missing.
enable without a selection keeps enabled agents and adds found ones, asking
first when there is a choice. Without a terminal it changes nothing, prints
the --only commands to run and exits with status 3 (also when nothing is found).
set and disable without a selection act on every enabled agent.
Claude: the policy works in the Claude desktop app's Code tab.
Status reports installed policy only; it does not fetch account usage.
`

// Run executes a CLI invocation. in supplies prompt answers; interactive reports
// whether a prompt may be shown. Callers map errors to exit codes.
func Run(args []string, in io.Reader, out io.Writer, interactive bool, version string) error {
	if len(args) == 0 || (len(args) == 1 && (isHelp(args[0]) || args[0] == "help")) {
		_, err := io.WriteString(out, help)
		return err
	}
	if len(args) == 1 && args[0] == "--version" {
		_, err := fmt.Fprintln(out, version)
		return err
	}
	inv, err := parse(args)
	if errors.Is(err, flag.ErrHelp) {
		_, err = io.WriteString(out, help)
		return err
	}
	if err != nil {
		return err
	}
	return inv.run(in, out, interactive)
}

func isHelp(arg string) bool { return arg == "--help" || arg == "-h" }

// invocation is a parsed command line. Slices are indexed like agents.
type invocation struct {
	command   string
	threshold int      // 0 keeps each installed threshold, or the default for a new block
	homes     []string // explicit home flags; "" when absent
	selected  []bool   // nil unless --only or a home flag selected agents
}

func parse(args []string) (invocation, error) {
	inv := invocation{command: args[0], homes: make([]string, len(agents))}
	args = args[1:]
	switch inv.command {
	case "enable", "set", "status", "disable":
	default:
		return inv, fmt.Errorf("unknown command %q; use --help", inv.command)
	}
	if len(args) == 1 && isHelp(args[0]) {
		return inv, flag.ErrHelp
	}
	if inv.command == "set" {
		if len(args) == 0 {
			return inv, errors.New("set requires an integer threshold from 1 to 99")
		}
		var err error
		if inv.threshold, err = parseThreshold(args[0]); err != nil {
			return inv, err
		}
		args = args[1:]
	}
	flags := flag.NewFlagSet(inv.command, flag.ContinueOnError)
	flags.SetOutput(io.Discard) // errors are returned, and help is printed by Run
	flags.Usage = func() {}
	threshold := ""
	if inv.command == "enable" {
		flags.StringVar(&threshold, "threshold", "", "remaining usage threshold (1-99)")
	}
	only := flags.String("only", "", "claude, codex or all")
	for i, a := range agents {
		flags.StringVar(&inv.homes[i], a.homeFlag, "", a.label+" home directory")
	}
	if err := flags.Parse(args); err != nil {
		return inv, err
	}
	if flags.NArg() != 0 {
		return inv, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	given := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { given[f.Name] = true })
	if given["threshold"] {
		var err error
		if inv.threshold, err = parseThreshold(threshold); err != nil {
			return inv, err
		}
	}
	if given["only"] {
		inv.selected = make([]bool, len(agents))
		for i, a := range agents {
			inv.selected[i] = *only == "all" || *only == a.name
		}
		if !slices.Contains(inv.selected, true) {
			return inv, fmt.Errorf("--only must be claude, codex or all, not %q", *only)
		}
	}
	// An explicit home flag selects its agent, as --only does.
	for i, a := range agents {
		if !given[a.homeFlag] {
			continue
		}
		if inv.homes[i] == "" {
			return inv, fmt.Errorf("--%s requires a path", a.homeFlag)
		}
		if given["only"] && !inv.selected[i] {
			return inv, fmt.Errorf("--%s conflicts with --only %s", a.homeFlag, *only)
		}
		if inv.selected == nil {
			inv.selected = make([]bool, len(agents))
		}
		inv.selected[i] = true
	}
	return inv, nil
}

// target is one agent with its resolved home.
type target struct {
	agent agent
	home  string
	found bool // the home exists
}

// candidates resolves the selected agents, or every agent when none is
// selected, in agents order.
func (inv invocation) candidates() ([]target, error) {
	var targets []target
	for i, a := range agents {
		if inv.selected != nil && !inv.selected[i] {
			continue
		}
		home, err := resolveHome(a, inv.homes[i])
		if err != nil {
			return nil, err
		}
		// Found means an existing directory; a file, a missing path or a
		// path below a file is not found.
		info, err := os.Stat(home)
		if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
			return nil, err
		}
		targets = append(targets, target{agent: a, home: home, found: err == nil && info.IsDir()})
	}
	return targets, nil
}

// resolveHome uses the flag, then a nonempty environment variable, then
// ~/<defaultDir>, made absolute.
func resolveHome(a agent, flagValue string) (string, error) {
	home := flagValue
	if home == "" {
		home = os.Getenv(a.homeEnv)
	}
	if home == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(user, a.defaultDir)
	}
	return filepath.Abs(home)
}

var errNothingEnabled = errors.New("the usage reserve is not enabled for Codex or Claude; run enable first")

func (inv invocation) run(in io.Reader, out io.Writer, interactive bool) error {
	candidates, err := inv.candidates()
	if err != nil {
		return err
	}
	// Selected agents are used even when their home is missing; otherwise only found ones.
	targets := candidates
	if inv.selected == nil {
		targets = nil
		for _, t := range candidates {
			if t.found {
				targets = append(targets, t)
			}
		}
		if len(targets) == 0 {
			switch inv.command {
			case "enable":
				var homes []string
				for _, t := range candidates {
					homes = append(homes, t.home)
				}
				return fmt.Errorf("%w (looked for %s); nothing was changed. Install Codex or Claude Code, then run agent-usage-reserve enable again", ErrNoAgent, strings.Join(homes, " and "))
			case "set":
				return errNothingEnabled
			}
			_, err = io.WriteString(out, "No Codex or Claude home found.\n")
			return err
		}
	}
	switch inv.command {
	case "status":
		failed := 0
		for _, t := range targets {
			res, err := inspect(t.agent, t.home)
			if err != nil && inv.selected == nil {
				// One broken agent must not hide the state of the others.
				failed++
				if err = writeFailure(out, t.agent, err); err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			if _, err = io.WriteString(out, statusLine(res)); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, "Installed policy only; account usage and compliance are not checked.\n"); err != nil {
			return err
		}
		return errFailed(failed)
	case "disable":
		if inv.selected != nil {
			// Disabling a selected agent without the policy changes nothing and still reports it.
			return applyEach(targets, "disable", 0, out, nil)
		}
		// Only enabled agents are changed, so a stale lock in a found home
		// that never had the policy cannot block the others. A failing agent
		// is reported and the rest are still processed.
		failed := 0
		for _, t := range targets {
			res, err := inspect(t.agent, t.home)
			if err == nil && res.enabled {
				res, err = apply(t.agent, t.home, "disable", 0)
			}
			if err != nil {
				failed++
				if err = writeFailure(out, t.agent, err); err != nil {
					return err
				}
				continue
			}
			if _, err = io.WriteString(out, statusLine(res)); err != nil {
				return err
			}
		}
		return errFailed(failed)
	case "set":
		// Check every target before changing any, so a refusal changes nothing.
		var enabled []target
		for _, t := range targets {
			res, err := inspect(t.agent, t.home)
			if err != nil {
				return inv.agentError(t, targets, err)
			}
			if res.enabled {
				enabled = append(enabled, t)
			} else if inv.selected != nil {
				return disabledError(t.agent)
			}
		}
		if len(enabled) == 0 {
			return errNothingEnabled
		}
		return applyEach(enabled, "set", inv.threshold, out, inv.failFast(targets))
	}
	if inv.selected == nil {
		if targets, err = inv.choose(targets, in, out, interactive); err != nil {
			return err
		}
	}
	return applyEach(targets, "enable", inv.threshold, out, inv.failFast(targets))
}

// failFast returns the error wrapper for enable and set without a selection,
// or nil when the agents were selected explicitly.
func (inv invocation) failFast(found []target) func(target, error) error {
	if inv.selected != nil {
		return nil
	}
	return func(t target, err error) error { return inv.agentError(t, found, err) }
}

// agentError names the failing agent and, when exactly one other agent was
// found, says how to run the command for that one alone.
func (inv invocation) agentError(failed target, found []target, err error) error {
	var others []target
	for _, t := range found {
		if t.agent.name != failed.agent.name {
			others = append(others, t)
		}
	}
	if len(others) != 1 {
		return fmt.Errorf("%s: %w", failed.agent.label, err)
	}
	command := "enable"
	if inv.command == "set" {
		command = fmt.Sprintf("set %d", inv.threshold)
	}
	return fmt.Errorf("%s: %w; to leave %s alone, run: agent-usage-reserve %s --only %s", failed.agent.label, err, failed.agent.label, command, others[0].agent.name)
}

// writeFailure prints the line for an agent that could not be inspected or changed.
func writeFailure(out io.Writer, a agent, err error) error {
	_, werr := fmt.Fprintf(out, "! %-*s  error: %s\n", labelWidth, a.label, err)
	return werr
}

// errFailed is the error returned after failed agents were reported line by line.
func errFailed(failed int) error {
	switch failed {
	case 0:
		return nil
	case 1:
		return errors.New("1 agent failed; see the line marked !")
	}
	return fmt.Errorf("%d agents failed; see the lines marked !", failed)
}

// choose applies the selection rules of enable without a selection to the
// found agents: enabled ones stay, a lone new agent is added without asking
// when nothing is enabled yet, and any other new agent needs an answer.
func (inv invocation) choose(found []target, in io.Reader, out io.Writer, interactive bool) ([]target, error) {
	var enabled, fresh []target
	for _, t := range found {
		res, err := inspect(t.agent, t.home)
		if err != nil {
			return nil, inv.agentError(t, found, err)
		}
		if res.enabled {
			enabled = append(enabled, t)
		} else {
			fresh = append(fresh, t)
		}
	}
	if len(fresh) == 0 || (len(fresh) == 1 && len(enabled) == 0) {
		return found, nil
	}
	answers := bufio.NewReader(in)
	keep := map[string]bool{}
	for _, t := range enabled {
		keep[t.agent.name] = true
	}
	if len(enabled) > 0 {
		// One new agent while another has the policy; with two agents,
		// keeping both means all of them.
		newcomer := fresh[0].agent
		situation := fmt.Sprintf("%s has the usage reserve and %s was found without it", enabled[0].agent.label, newcomer.label)
		only := []string{"all", enabled[0].agent.name}
		if !interactive {
			return nil, inv.needsChoice(situation, "no terminal is available to ask", only)
		}
		answer, err := ask(answers, out, "Also enable for "+newcomer.label+"? [Y/n]: ", "Please answer y or n.\n",
			func(s string) bool { return slices.Contains([]string{"", "y", "yes", "n", "no"}, s) })
		if err != nil {
			return nil, inv.askFailure(err, situation, only)
		}
		keep[newcomer.name] = answer == "" || answer[0] == 'y'
	} else {
		// Both agents found and neither enabled.
		situation := "found Codex and Claude"
		only := []string{"all"}
		for _, t := range fresh {
			only = append(only, t.agent.name)
		}
		if !interactive {
			return nil, inv.needsChoice(situation, "no terminal is available to ask", only)
		}
		menu := "Found Codex and Claude. Apply the usage reserve to:\n  1) Both\n  2) Claude only\n  3) Codex only\n"
		if _, err := io.WriteString(out, menu); err != nil {
			return nil, err
		}
		answer, err := ask(answers, out, "Choose [1]: ", "Please enter 1, 2 or 3.\n",
			func(s string) bool { return slices.Contains([]string{"", "1", "2", "3"}, s) })
		if err != nil {
			return nil, inv.askFailure(err, situation, only)
		}
		keep["claude"] = answer != "3"
		keep["codex"] = answer != "2"
	}
	var chosen []target
	for _, t := range found {
		if keep[t.agent.name] {
			chosen = append(chosen, t)
		}
	}
	return chosen, nil
}

// errNoAnswer means input ended or could not be read before a valid answer.
var errNoAnswer = errors.New("no answer was given")

// errInvalidAnswers means three answers in a row were not accepted.
var errInvalidAnswers = errors.New("no valid answer was given")

// ask writes question and reads one answer per line, normalized to lower case
// without surrounding space, until valid accepts it. It gives up after three
// invalid answers with errInvalidAnswers, and returns errNoAnswer when input
// ends or cannot be read first. Any other error is a failure to write.
func ask(answers *bufio.Reader, out io.Writer, question, retry string, valid func(string) bool) (string, error) {
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			if _, err := io.WriteString(out, retry); err != nil {
				return "", err
			}
		}
		if _, err := io.WriteString(out, question); err != nil {
			return "", err
		}
		line, err := answers.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			// End the prompt line so the error starts on its own line.
			if _, werr := io.WriteString(out, "\n"); werr != nil {
				return "", werr
			}
			return "", fmt.Errorf("%w (%v)", errNoAnswer, err)
		}
		if line == "" {
			if _, err := io.WriteString(out, "\n"); err != nil {
				return "", err
			}
			return "", errNoAnswer
		}
		if answer := strings.ToLower(strings.TrimSpace(line)); valid(answer) {
			return answer, nil
		}
	}
	return "", errInvalidAnswers
}

// askFailure turns a prompt that got no usable answer into ErrNeedsChoice
// with the --only commands; nothing has been changed at that point.
func (inv invocation) askFailure(err error, situation string, only []string) error {
	if errors.Is(err, errNoAnswer) || errors.Is(err, errInvalidAnswers) {
		return inv.needsChoice(situation, err.Error(), only)
	}
	return err
}

// needsChoice wraps ErrNeedsChoice with the --only commands that make the choice.
func (inv invocation) needsChoice(situation, reason string, only []string) error {
	var commands strings.Builder
	for _, value := range only {
		fmt.Fprintf(&commands, "\n  agent-usage-reserve enable --only %s", value)
		if inv.threshold != 0 {
			fmt.Fprintf(&commands, " --threshold %d", inv.threshold)
		}
	}
	return fmt.Errorf("%s; %w but %s, so nothing was changed. Run one of:%s", situation, ErrNeedsChoice, reason, commands.String())
}

// applyEach runs command for each target in order, printing its line, and
// stops at the first error, passed through wrap when it is not nil.
func applyEach(targets []target, command string, threshold int, out io.Writer, wrap func(target, error) error) error {
	for _, t := range targets {
		res, err := apply(t.agent, t.home, command, threshold)
		if err != nil {
			if wrap != nil {
				err = wrap(t, err)
			}
			return err
		}
		if _, err = io.WriteString(out, statusLine(res)); err != nil {
			return err
		}
	}
	return nil
}

// labelWidth aligns the state column across agents.
var labelWidth = func() int {
	width := 0
	for _, a := range agents {
		width = max(width, len(a.label))
	}
	return width
}()

func statusLine(r result) string {
	mark, state := "–", "disabled"
	if r.enabled {
		mark, state = "✓", fmt.Sprintf("%d%%", r.threshold)
	}
	line := fmt.Sprintf("%s %-*s  %s  %s", mark, labelWidth, r.agent.label, state, r.target)
	if r.agent.note != "" {
		line += "  (" + r.agent.note + ")"
	}
	return line + "\n"
}
