// Package reserve manages only its marked section in global Codex instructions.
package reserve

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
	return execute(command, absolute, threshold, out)
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

const markerPrefix = "<!-- codex-usage-reserve:managed:"
const endMarker = "<!-- codex-usage-reserve:managed:v1:end -->\n"

var beginPattern = regexp.MustCompile(`^<!-- codex-usage-reserve:managed:v1:begin threshold=([0-9]+) padding=([0-2]) -->\n`)

type block struct{ start, end, threshold int }

func findBlock(data []byte) (*block, error) {
	count := bytes.Count(data, []byte(markerPrefix))
	if count == 0 {
		return nil, nil
	}
	if count != 2 {
		return nil, errors.New("duplicate or malformed managed policy markers; restore from backup or repair the marked block manually")
	}
	start := bytes.Index(data, []byte(markerPrefix))
	match := beginPattern.FindSubmatch(data[start:])
	if match == nil {
		return nil, errors.New("malformed or unsupported managed policy marker")
	}
	threshold, err := parseThreshold(string(match[1]))
	if err != nil {
		return nil, err
	}
	padding, _ := strconv.Atoi(string(match[2]))
	if start < padding {
		return nil, errors.New("malformed managed policy padding")
	}
	for _, b := range data[start-padding : start] {
		if b != '\n' {
			return nil, errors.New("managed policy separator changed; refusing to remove unrelated bytes")
		}
	}
	if start > 0 && data[start-1] != '\n' {
		return nil, errors.New("managed policy marker is not on its own line")
	}
	end := bytes.Index(data[start+len(match[0]):], []byte(endMarker))
	if end < 0 {
		return nil, errors.New("missing managed policy end marker")
	}
	end += start + len(match[0])
	if end > 0 && data[end-1] != '\n' {
		return nil, errors.New("managed policy end marker is not on its own line")
	}
	return &block{start - padding, end + len(endMarker), threshold}, nil
}

func policy(threshold, padding int) []byte {
	return []byte(strings.Repeat("\n", padding) + fmt.Sprintf(`<!-- codex-usage-reserve:managed:v1:begin threshold=%d padding=%d -->
## Account usage reserve

Before ordinary work and between major stages, check available Codex account usage limits with get_usage_limits or its namespaced equivalent. Prefer rateLimitsByLimitId over legacy rateLimits. Check the actual rate-limit windows applicable to the current task and model; do not stop because of unrelated special-model buckets. For each available window, remaining percent is 100 minus usedPercent, clamped to 0–100. Null or missing values/windows are unavailable, not zero usage.

If any applicable window has %d%% or less remaining, checkpoint progress safely and stop ordinary work. Tell the user the measured window and remaining percentage, that work stopped to reserve usage for scheduled tasks, the current progress, and how to resume.

Actual scheduled task runs are exempt from this stopping threshold; unrelated manual work is not. If the user explicitly asks to continue, resume only the specified task (or the current task if unspecified). Follow a user-specified lower temporary threshold; otherwise that task may finish. This permission does not change the default threshold for other tasks.

If the usage tool or data is unavailable, disclose that fact without guessing remaining usage, claiming the threshold was reached, or inventing another blocking rule. This is a behavioral operating policy, not a hard account cap; it cannot guarantee an exact reserve or scheduled-task success.
<!-- codex-usage-reserve:managed:v1:end -->
`, threshold, padding, threshold))
}

type snapshot struct {
	path string
	data []byte
	info os.FileInfo
}

func readSnapshot(path string) (snapshot, error) {
	s := snapshot{path: path}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return s, fmt.Errorf("refusing symlink instruction target: %s", path)
	}
	if !info.Mode().IsRegular() {
		return s, fmt.Errorf("instruction target is not a regular file: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	// File.Stat captures identity from the open handle. On Windows, Lstat
	// may defer looking up identity by pathname until a later SameFile call.
	s.info, err = f.Stat()
	if err != nil {
		return s, err
	}
	if !s.info.Mode().IsRegular() {
		return s, fmt.Errorf("instruction target is not a regular file: %s", path)
	}
	s.data, err = io.ReadAll(f)
	closeErr := f.Close()
	if err = errors.Join(err, closeErr); err != nil {
		return s, err
	}
	// Close before SameFile: Windows' lazy pathname identity lookup otherwise
	// conflicts with the open read handle. The handle-derived identity stays valid.
	// Reject a link or replacement observed after reading the opened target.
	current, err := os.Lstat(path)
	if err != nil {
		return s, err
	}
	if current.Mode()&os.ModeSymlink != 0 {
		return s, fmt.Errorf("refusing symlink instruction target: %s", path)
	}
	if !os.SameFile(s.info, current) {
		return s, fmt.Errorf("external file replacement detected while reading %s", path)
	}
	return s, nil
}
func readTargets(home string) ([2]snapshot, int, *block, error) {
	var targets [2]snapshot
	for i, name := range []string{"AGENTS.md", "AGENTS.override.md"} {
		s, err := readSnapshot(filepath.Join(home, name))
		if err != nil {
			return targets, 0, nil, err
		}
		targets[i] = s
	}
	active := 0
	if len(targets[1].data) > 0 {
		active = 1
	}
	var found *block
	for i, s := range targets {
		b, err := findBlock(s.data)
		if err != nil {
			return targets, active, nil, fmt.Errorf("%s: %w", s.path, err)
		}
		if b != nil {
			if i != active {
				return targets, active, nil, fmt.Errorf("managed policy exists in inactive %s; disable it while that file is effective, then enable again after changing override precedence (or manually relocate the entire marked region)", s.path)
			}
			found = b
		}
	}
	return targets, active, found, nil
}

func execute(command, home string, threshold int, out io.Writer) error {
	info, err := os.Lstat(home)
	if os.IsNotExist(err) {
		if command != "enable" {
			if command == "set" {
				return errors.New("policy is disabled; run enable first")
			}
			_, err = fmt.Fprintf(out, "Disabled: %s\n", filepath.Join(home, "AGENTS.md"))
			return err
		}
		if err = os.MkdirAll(home, 0700); err != nil {
			return err
		}
		info, err = os.Lstat(home)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink Codex home: %s", home)
	}
	if !info.IsDir() {
		return fmt.Errorf("Codex home is not a directory: %s", home)
	}
	// Resolve ancestor aliases (such as macOS /var) once; direct home links are refused above.
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return err
	}
	if command != "status" {
		unlock, err := acquireLock(home)
		if err != nil {
			return err
		}
		defer unlock()
	}
	targets, active, b, err := readTargets(home)
	if err != nil {
		return err
	}
	target := targets[active]
	if command == "status" {
		if b == nil {
			_, err = fmt.Fprintf(out, "Disabled: %s\n", target.path)
		} else {
			_, err = fmt.Fprintf(out, "Enabled: %d%% remaining threshold\nTarget: %s\nInstalled policy only; account usage and compliance are not checked.\n", b.threshold, target.path)
		}
		return err
	}
	if command == "set" && b == nil {
		return errors.New("policy is disabled; run enable first")
	}
	if command == "disable" && b == nil {
		_, err = fmt.Fprintf(out, "Already disabled: %s\n", target.path)
		return err
	}
	var next []byte
	if command == "disable" {
		next = append(append([]byte{}, target.data[:b.start]...), target.data[b.end:]...)
	} else {
		if threshold == 0 {
			threshold = 10
			if b != nil {
				threshold = b.threshold
			}
		}
		if b != nil {
			padding := 0
			headerStart := bytes.Index(target.data[b.start:b.end], []byte(markerPrefix))
			padding = headerStart
			next = append(append(append([]byte{}, target.data[:b.start]...), policy(threshold, padding)...), target.data[b.end:]...)
		} else {
			padding := 0
			if len(target.data) > 0 {
				padding = 2
				if target.data[len(target.data)-1] == '\n' {
					padding = 1
				}
			}
			next = append(append([]byte{}, target.data...), policy(threshold, padding)...)
		}
	}
	if !bytes.Equal(next, target.data) {
		if err = replace(targets, active, next); err != nil {
			return err
		}
	}
	if command == "disable" {
		_, err = fmt.Fprintf(out, "Disabled: %s\n", target.path)
	} else {
		_, err = fmt.Fprintf(out, "Enabled: %d%% remaining threshold\nTarget: %s\n", threshold, target.path)
	}
	return err
}

func acquireLock(home string) (func(), error) {
	path := filepath.Join(home, ".codex-usage-reserve.lock")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot acquire lock %s (another operation may be running; inspect a stale lock before removing it): %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		os.Remove(path)
		return nil, err
	}
	f.Close()
	return func() {
		current, e := os.Lstat(path)
		if e == nil && os.SameFile(info, current) {
			os.Remove(path)
		}
	}, nil
}

func unchanged(before snapshot) error {
	after, err := readSnapshot(before.path)
	if err != nil {
		return err
	}
	if (before.info == nil) != (after.info == nil) || !bytes.Equal(before.data, after.data) {
		return fmt.Errorf("external edit detected in %s; retry after reviewing the file", before.path)
	}
	if before.info != nil && (!os.SameFile(before.info, after.info) || before.info.Mode() != after.info.Mode()) {
		return fmt.Errorf("external file replacement or permission change detected in %s", before.path)
	}
	return nil
}

func replace(targets [2]snapshot, active int, next []byte) error {
	target := targets[active]
	home := filepath.Dir(target.path)
	if target.info != nil {
		backup, err := os.CreateTemp(home, ".codex-usage-reserve-backup-*")
		if err != nil {
			return err
		}
		_, writeErr := backup.Write(target.data)
		syncErr := backup.Sync()
		closeErr := backup.Close()
		if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
	}
	temp, err := os.CreateTemp(home, ".codex-usage-reserve-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	mode := os.FileMode(0600)
	if target.info != nil {
		mode = target.info.Mode().Perm()
	}
	if err = temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	_, writeErr := temp.Write(next)
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	for _, before := range targets {
		if err = unchanged(before); err != nil {
			return err
		}
	}
	if err = os.Rename(temp.Name(), target.path); err != nil {
		return fmt.Errorf("atomic replacement failed: %w", err)
	}
	return nil
}
