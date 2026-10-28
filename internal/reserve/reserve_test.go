package reserve

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func run(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	args = append(args, "--codex-home", home)
	err := Run(args, &out, "test")
	return out.String(), err
}
func read(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func write(t *testing.T, path string, b []byte) {
	t.Helper()
	if e := os.WriteFile(path, b, 0640); e != nil {
		t.Fatal(e)
	}
}

// Catches lifecycle state errors and destructive rewriting of unrelated bytes.
func TestLifecyclePreservesBytes(t *testing.T) {
	for _, original := range []string{"", "hello", "hello\n", "hello\r\nworld\r\n"} {
		t.Run(strings.ReplaceAll(original, "\n", "N"), func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "AGENTS.md")
			write(t, path, []byte(original))
			if _, e := run(t, home, "enable", "--threshold", "15"); e != nil {
				t.Fatal(e)
			}
			installed := read(t, path)
			if !bytes.HasPrefix(installed, []byte(original)) {
				t.Fatal("original bytes changed")
			}
			if out, e := run(t, home, "status"); e != nil || !strings.Contains(out, "15%") {
				t.Fatalf("%s %v", out, e)
			}
			if _, e := run(t, home, "enable"); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(installed, read(t, path)) {
				t.Fatal("repeat enable changed policy")
			}
			if _, e := run(t, home, "set", "20"); e != nil {
				t.Fatal(e)
			}
			if out, e := run(t, home, "status"); e != nil || !strings.Contains(out, "20%") {
				t.Fatalf("%s %v", out, e)
			}
			f, e := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if e != nil {
				t.Fatal(e)
			}
			f.WriteString("user addition\r\n")
			f.Close()
			if _, e := run(t, home, "disable"); e != nil {
				t.Fatal(e)
			}
			if got := string(read(t, path)); got != original+"user addition\r\n" {
				t.Fatalf("roundtrip %q", got)
			}
			if _, e := run(t, home, "disable"); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestOverrideSelectionAndInactivePolicy(t *testing.T) {
	home := t.TempDir()
	regular := filepath.Join(home, "AGENTS.md")
	override := filepath.Join(home, "AGENTS.override.md")
	write(t, regular, []byte("regular"))
	write(t, override, []byte("override"))
	if _, e := run(t, home, "enable"); e != nil {
		t.Fatal(e)
	}
	if string(read(t, regular)) != "regular" {
		t.Fatal("wrong target")
	}
	write(t, override, nil)
	// Move the installed policy into the now inactive override.
	if _, e := run(t, home, "enable"); e != nil {
		t.Fatal(e)
	}
	installed := read(t, regular)
	write(t, regular, []byte("regular"))
	write(t, override, append(installed, []byte("\n")...))
	if _, e := run(t, home, "disable"); e != nil {
		t.Fatal(e)
	}
	// A regular-file policy becomes inactive when a nonempty override appears.
	write(t, override, nil)
	if _, e := run(t, home, "enable"); e != nil {
		t.Fatal(e)
	}
	write(t, override, []byte("new override"))
	if _, e := run(t, home, "enable"); e == nil || !strings.Contains(e.Error(), "inactive") {
		t.Fatalf("expected inactive conflict: %v", e)
	}
}

func TestValidationAndReadOnlyMissingHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "missing")
	for _, args := range [][]string{{"set", "15"}, {"enable", "--threshold", "0"}, {"enable", "--threshold", "100"}, {"enable", "--threshold", "1.5"}, {"set"}, {"status", "extra"}, {"enable", "--unknown"}} {
		if _, e := run(t, home, args...); e == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, cmd := range []string{"status", "disable"} {
		if _, e := run(t, home, cmd); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := os.Stat(home); !os.IsNotExist(e) {
		t.Fatal("read-only command created directory")
	}
}

func TestMalformedMarkersAndSymlinks(t *testing.T) {
	for _, content := range []string{"<!-- codex-usage-reserve:managed:v1:begin -->", "<!-- codex-usage-reserve:managed:v2:begin -->"} {
		home := t.TempDir()
		path := filepath.Join(home, "AGENTS.md")
		write(t, path, []byte(content))
		if _, e := run(t, home, "enable"); e == nil {
			t.Fatal("accepted malformed policy")
		}
		if string(read(t, path)) != content {
			t.Fatal("changed malformed policy")
		}
	}
	home := t.TempDir()
	path := filepath.Join(home, "AGENTS.md")
	destination := filepath.Join(t.TempDir(), "target")
	write(t, destination, []byte("private"))
	if e := os.Symlink(destination, path); e != nil {
		t.Skipf("symlink unavailable: %v", e)
	}
	if _, e := run(t, home, "enable"); e == nil {
		t.Fatal("followed target symlink")
	}
	if string(read(t, destination)) != "private" {
		t.Fatal("modified symlink destination")
	}
}

func TestLockBackupAndMode(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "AGENTS.md")
	write(t, path, []byte("original"))
	lock := filepath.Join(home, ".codex-usage-reserve.lock")
	write(t, lock, []byte("owner"))
	if _, e := run(t, home, "enable"); e == nil {
		t.Fatal("ignored lock")
	}
	if string(read(t, lock)) != "owner" {
		t.Fatal("deleted another lock")
	}
	os.Remove(lock)
	if _, e := run(t, home, "enable"); e != nil {
		t.Fatal(e)
	}
	backups, e := filepath.Glob(filepath.Join(home, ".codex-usage-reserve-backup-*"))
	if e != nil || len(backups) != 1 {
		t.Fatalf("backups %v %v", backups, e)
	}
	if string(read(t, backups[0])) != "original" {
		t.Fatal("wrong backup")
	}
	info, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		t.Fatalf("mode %v", info.Mode())
	}
	info, e = os.Stat(backups[0])
	if e != nil {
		t.Fatal(e)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatal("backup is not private")
	}
}

// Catches lost updates from file edits or override precedence changes after reading.
func TestExternalEditsAreNotOverwritten(t *testing.T) {
	for _, change := range []string{"contents", "replacement", "override"} {
		t.Run(change, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "AGENTS.md")
			write(t, path, []byte("before"))
			targets, active, _, e := readTargets(home)
			if e != nil {
				t.Fatal(e)
			}
			switch change {
			case "contents":
				write(t, path, []byte("external"))
			case "replacement":
				if e = os.Rename(path, path+".old"); e != nil {
					t.Fatal(e)
				}
				write(t, path, []byte("before"))
			case "override":
				write(t, filepath.Join(home, "AGENTS.override.md"), []byte("override"))
			}
			if e = replace(targets, active, []byte("clobbered")); e == nil {
				t.Fatal("overwrote external change")
			}
			if string(read(t, path)) == "clobbered" {
				t.Fatal("lost update")
			}
		})
	}
}

func TestDuplicateBlocksAndUnmarkedPolicy(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "AGENTS.md")
	original := "Keep my existing reserve policy at 7%."
	write(t, path, []byte(original))
	if _, e := run(t, home, "enable"); e != nil {
		t.Fatal(e)
	}
	installed := read(t, path)
	write(t, path, append(append([]byte{}, installed...), installed...))
	for _, command := range []string{"enable", "disable", "status"} {
		if _, e := run(t, home, command); e == nil {
			t.Fatalf("%s accepted duplicate blocks", command)
		}
	}
	write(t, path, installed)
	if _, e := run(t, home, "disable"); e != nil {
		t.Fatal(e)
	}
	if string(read(t, path)) != original {
		t.Fatal("changed unmarked policy")
	}
}

func TestExplicitThresholdEnvironmentAndVersion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	var out bytes.Buffer
	if e := Run([]string{"enable"}, &out, "dev"); e != nil {
		t.Fatal(e)
	}
	if _, e := run(t, home, "enable", "--threshold=17"); e != nil {
		t.Fatal(e)
	}
	if output, e := run(t, home, "status"); e != nil || !strings.Contains(output, "17%") {
		t.Fatalf("%s %v", output, e)
	}
	out.Reset()
	if e := Run([]string{"--version"}, &out, "0.1.0"); e != nil || out.String() != "0.1.0\n" {
		t.Fatalf("%q %v", out.String(), e)
	}
	explicit := t.TempDir()
	if _, e := run(t, explicit, "enable"); e != nil {
		t.Fatal(e)
	}
	if output, e := run(t, explicit, "status"); e != nil || !strings.Contains(output, "10%") {
		t.Fatalf("%s %v", output, e)
	}
}

func TestHomeSymlinkAndAncestorAlias(t *testing.T) {
	home := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if e := os.Symlink(home, alias); e != nil {
		t.Skipf("symlink unavailable: %v", e)
	}
	if _, e := run(t, alias, "enable"); e == nil {
		t.Fatal("accepted direct home symlink")
	}
	child := filepath.Join(alias, "child")
	if _, e := run(t, child, "enable"); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(home, "child", "AGENTS.md")); e != nil {
		t.Fatal(e)
	}
}

func TestReplacedLockIsNotRemoved(t *testing.T) {
	home := t.TempDir()
	unlock, e := acquireLock(home)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(home, ".codex-usage-reserve.lock")
	// Keep the original inode alive so inode reuse cannot hide the replacement.
	if e = os.Rename(path, path+".old"); e != nil {
		t.Fatal(e)
	}
	write(t, path, []byte("new owner"))
	unlock()
	if string(read(t, path)) != "new owner" {
		t.Fatal("removed new owner's lock")
	}
}

func TestSetHelp(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"set", "--help"}, &out, "dev"); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	if !strings.Contains(out.String(), "set 15") {
		t.Fatal("missing set usage")
	}
}

// On Windows, pathname-based FileInfo can otherwise acquire the replacement's
// identity only when SameFile is first called, losing the original snapshot.
func TestSnapshotRetainsOriginalFileIdentity(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "AGENTS.md")
	write(t, path, []byte("same bytes"))
	before, err := readSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	write(t, path, []byte("same bytes"))
	original, err := os.Open(path + ".old")
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	originalInfo, err := original.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before.info, originalInfo) {
		t.Fatal("snapshot identity changed when pathname was replaced")
	}
	if err = unchanged(before); err == nil {
		t.Fatal("same-byte file replacement accepted")
	}
}

func TestSymlinkSubstitutedAfterSnapshotIsRefused(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "AGENTS.md")
	write(t, path, []byte("same bytes"))
	targets, active, _, err := readTargets(home)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(path+".old", path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err = replace(targets, active, []byte("clobbered")); err == nil {
		t.Fatal("accepted substituted target symlink")
	}
	if string(read(t, path+".old")) != "same bytes" {
		t.Fatal("changed symlink destination")
	}
}
