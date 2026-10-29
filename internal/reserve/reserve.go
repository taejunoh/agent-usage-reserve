// Package reserve manages only its marked section in global agent instructions.
package reserve

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const help = `Usage:
  codex-usage-reserve enable [--threshold 10] [--codex-home PATH]
  codex-usage-reserve set 15 [--codex-home PATH]
  codex-usage-reserve status [--codex-home PATH]
  codex-usage-reserve disable [--codex-home PATH]
  codex-usage-reserve --version

Home: --codex-home, then CODEX_HOME, then ~/.codex.
Status reports installed policy only; it does not fetch account usage.
`

// Run executes a CLI invocation; callers decide how to display returned errors.
func Run(args []string, out io.Writer, version string) error {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help")) {
		_, err := io.WriteString(out, help)
		return err
	}
	if len(args) == 1 && args[0] == "--version" {
		_, err := fmt.Fprintln(out, version)
		return err
	}
	command := args[0]
	args = args[1:]
	if command != "enable" && command != "set" && command != "status" && command != "disable" {
		return fmt.Errorf("unknown command %q; use --help", command)
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		_, err := io.WriteString(out, help)
		return err
	}
	threshold := 0
	if command == "set" {
		if len(args) == 0 {
			return errors.New("set requires an integer threshold from 1 to 99")
		}
		var err error
		threshold, err = parseThreshold(args[0])
		if err != nil {
			return err
		}
		args = args[1:]
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(out)
	home := fs.String("codex-home", "", "Codex home directory")
	thresholdArg := ""
	if command == "enable" {
		fs.StringVar(&thresholdArg, "threshold", "", "remaining usage threshold (1-99)")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	specified := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "threshold" {
			specified = true
		}
	})
	if specified {
		var err error
		threshold, err = parseThreshold(thresholdArg)
		if err != nil {
			return err
		}
	}
	if *home == "" {
		*home = os.Getenv("CODEX_HOME")
	}
	if *home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		*home = filepath.Join(h, ".codex")
	}
	absolute, err := filepath.Abs(*home)
	if err != nil {
		return err
	}
	// Single-agent output of the initial release; Task 2 replaces it.
	if command == "status" {
		res, err := inspect(codexAgent, absolute)
		if err != nil {
			return err
		}
		if !res.enabled {
			_, err = fmt.Fprintf(out, "Disabled: %s\n", res.target)
		} else {
			_, err = fmt.Fprintf(out, "Enabled: %d%% remaining threshold\nTarget: %s\nInstalled policy only; account usage and compliance are not checked.\n", res.threshold, res.target)
		}
		return err
	}
	homeExisted := false
	if command == "disable" {
		// A missing home reports "Disabled"; an existing one without a block, "Already disabled".
		_, statErr := os.Lstat(absolute)
		homeExisted = statErr == nil
	}
	res, err := apply(codexAgent, absolute, command, threshold)
	if err != nil {
		return err
	}
	switch {
	case command != "disable":
		_, err = fmt.Fprintf(out, "Enabled: %d%% remaining threshold\nTarget: %s\n", res.threshold, res.target)
	case homeExisted && !res.changed:
		_, err = fmt.Fprintf(out, "Already disabled: %s\n", res.target)
	default:
		_, err = fmt.Fprintf(out, "Disabled: %s\n", res.target)
	}
	return err
}

func parseThreshold(s string) (int, error) {
	if s == "" || strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, errors.New("threshold must be an integer from 1 to 99")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 99 {
		return 0, errors.New("threshold must be an integer from 1 to 99")
	}
	return n, nil
}
