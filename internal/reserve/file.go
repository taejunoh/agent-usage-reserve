package reserve

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const markerPrefix = "<!-- agent-usage-reserve:managed:"
const endMarker = "<!-- agent-usage-reserve:managed:v1:end -->\n"

var beginPattern = regexp.MustCompile(`^<!-- agent-usage-reserve:managed:v1:begin agent=([a-z]+) threshold=([0-9]+) padding=([0-2]) -->\n`)

// Markers written by codex-usage-reserve 0.1.0, recognized in Codex files only.
const legacyMarkerPrefix = "<!-- codex-usage-reserve:managed:"
const legacyEndMarker = "<!-- codex-usage-reserve:managed:v1:end -->\n"

var legacyBeginPattern = regexp.MustCompile(`^<!-- codex-usage-reserve:managed:v1:begin threshold=([0-9]+) padding=([0-2]) -->\n`)

// block spans the managed region, including its recorded leading newlines.
type block struct {
	start, end, threshold, padding int
	legacy                         bool
}

// result is the state of one agent's policy after apply or inspect.
type result struct {
	agent     agent
	enabled   bool
	threshold int
	target    string // absolute path of the effective file
	changed   bool
}

func findBlock(data []byte, a agent) (*block, error) {
	current := bytes.Count(data, []byte(markerPrefix))
	legacy := bytes.Count(data, []byte(legacyMarkerPrefix))
	if legacy > 0 && !a.legacy {
		return nil, fmt.Errorf("codex-usage-reserve markers are supported only in Codex instructions, not %s; remove the marked block manually", a.label)
	}
	if current+legacy == 0 {
		return nil, nil
	}
	if current+legacy != 2 || (current > 0 && legacy > 0) {
		return nil, errors.New("duplicate or malformed managed policy markers; restore from backup or repair the marked block manually")
	}
	prefix, pattern, end := markerPrefix, beginPattern, endMarker
	if legacy > 0 {
		prefix, pattern, end = legacyMarkerPrefix, legacyBeginPattern, legacyEndMarker
	}
	start := bytes.Index(data, []byte(prefix))
	match := pattern.FindSubmatch(data[start:])
	if match == nil {
		return nil, errors.New("malformed or unsupported managed policy marker")
	}
	fields := match[1:]
	if legacy == 0 {
		if owner := string(fields[0]); owner != a.name {
			return nil, fmt.Errorf("managed policy marker belongs to agent %q, not %q; remove the marked block manually", owner, a.name)
		}
		fields = fields[1:]
	}
	threshold, err := parseThreshold(string(fields[0]))
	if err != nil {
		return nil, err
	}
	padding, _ := strconv.Atoi(string(fields[1]))
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
	stop := bytes.Index(data[start+len(match[0]):], []byte(end))
	if stop < 0 {
		return nil, errors.New("missing managed policy end marker")
	}
	stop += start + len(match[0])
	if stop > 0 && data[stop-1] != '\n' {
		return nil, errors.New("managed policy end marker is not on its own line")
	}
	return &block{start: start - padding, end: stop + len(end), threshold: threshold, padding: padding, legacy: legacy > 0}, nil
}

// managedBlock renders the current-format block; padding is the count of
// leading newlines recorded so disable removes exactly what enable added.
func managedBlock(a agent, threshold, padding int) []byte {
	return []byte(strings.Repeat("\n", padding) +
		fmt.Sprintf("<!-- agent-usage-reserve:managed:v1:begin agent=%s threshold=%d padding=%d -->\n", a.name, threshold, padding) +
		a.policy(threshold) + endMarker)
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

// readTargets snapshots every candidate file. The last nonempty one is
// effective; a managed block anywhere else is refused.
func readTargets(a agent, home string) ([]snapshot, int, *block, error) {
	targets := make([]snapshot, len(a.targets))
	for i, name := range a.targets {
		s, err := readSnapshot(filepath.Join(home, name))
		if err != nil {
			return targets, 0, nil, err
		}
		targets[i] = s
	}
	active := 0
	for i, s := range targets {
		if len(s.data) > 0 {
			active = i
		}
	}
	var found *block
	for i, s := range targets {
		b, err := findBlock(s.data, a)
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

// openHome validates an agent home and resolves ancestor aliases. A missing
// home is created only when create is set; otherwise exists is false.
func openHome(a agent, home string, create bool) (string, bool, error) {
	home, err := filepath.Abs(home)
	if err != nil {
		return "", false, err
	}
	info, err := os.Lstat(home)
	if os.IsNotExist(err) {
		if !create {
			return home, false, nil
		}
		if err = os.MkdirAll(home, 0700); err != nil {
			return "", false, err
		}
		info, err = os.Lstat(home)
	}
	if err != nil {
		return "", false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", false, fmt.Errorf("refusing symlink %s home: %s", a.label, home)
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("%s home is not a directory: %s", a.label, home)
	}
	// Resolve ancestor aliases (such as macOS /var) once; direct home links are refused above.
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return "", false, err
	}
	return home, true, nil
}

// inspect reports state without locking or writing; a missing home is disabled.
func inspect(a agent, home string) (result, error) {
	res := result{agent: a}
	home, exists, err := openHome(a, home, false)
	if err != nil {
		return res, err
	}
	if !exists {
		res.target = filepath.Join(home, a.targets[0])
		return res, nil
	}
	targets, active, b, err := readTargets(a, home)
	if err != nil {
		return res, err
	}
	res.target = targets[active].path
	if b != nil {
		res.enabled, res.threshold = true, b.threshold
	}
	return res, nil
}

// apply runs "enable", "set" or "disable" for one agent home. threshold 0
// means keep the installed threshold, or the default 10 for a new block.
func apply(a agent, home, command string, threshold int) (result, error) {
	res := result{agent: a}
	if command != "enable" && command != "set" && command != "disable" {
		return res, fmt.Errorf("unknown command %q", command)
	}
	if threshold < 0 || threshold > 99 {
		return res, errors.New("threshold must be an integer from 1 to 99")
	}
	home, exists, err := openHome(a, home, command == "enable")
	if err != nil {
		return res, err
	}
	if !exists {
		if command == "set" {
			return res, errors.New("policy is disabled; run enable first")
		}
		res.target = filepath.Join(home, a.targets[0])
		return res, nil
	}
	unlock, err := acquireLock(home)
	if err != nil {
		return res, err
	}
	defer unlock()
	targets, active, b, err := readTargets(a, home)
	if err != nil {
		return res, err
	}
	target := targets[active]
	res.target = target.path
	if command == "set" && b == nil {
		return res, errors.New("policy is disabled; run enable first")
	}
	if command == "disable" && b == nil {
		return res, nil
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
			// Rewrites a legacy block to the current marker, keeping its padding.
			next = append(append(append([]byte{}, target.data[:b.start]...), managedBlock(a, threshold, b.padding)...), target.data[b.end:]...)
		} else {
			padding := 0
			if len(target.data) > 0 {
				padding = 2
				if target.data[len(target.data)-1] == '\n' {
					padding = 1
				}
			}
			next = append(append([]byte{}, target.data...), managedBlock(a, threshold, padding)...)
		}
		res.enabled, res.threshold = true, threshold
	}
	if !bytes.Equal(next, target.data) {
		if err = replace(targets, active, next); err != nil {
			return res, err
		}
		res.changed = true
	}
	return res, nil
}

func acquireLock(home string) (func(), error) {
	path := filepath.Join(home, ".agent-usage-reserve.lock")
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

func replace(targets []snapshot, active int, next []byte) error {
	target := targets[active]
	home := filepath.Dir(target.path)
	if target.info != nil {
		backup, err := os.CreateTemp(home, ".agent-usage-reserve-backup-*")
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
	temp, err := os.CreateTemp(home, ".agent-usage-reserve-write-*")
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
