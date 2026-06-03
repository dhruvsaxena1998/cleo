package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const remoteTestPubKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITESTKEY phone@iphone"

// remoteTestKeyBody is the "<type> <base64>" prefix the managed line keeps
// after dropping the trailing comment.
const remoteTestKeyBody = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITESTKEY"

func runRemote(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := NewRootCmd(func(*Ctx) error { return nil })
	cmd.SetArgs(args)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetIn(strings.NewReader(stdin))
	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

func assertManagedLine(t *testing.T, out string) {
	t.Helper()
	if !strings.HasPrefix(out, `restrict,pty,command="`) {
		t.Errorf("expected forced-command prefix, got %q", out)
	}
	if !strings.Contains(out, remoteTestKeyBody) {
		t.Errorf("expected key body %q in output %q", remoteTestKeyBody, out)
	}
	if strings.Contains(out, "phone@iphone") {
		t.Errorf("key comment should be dropped, got %q", out)
	}
	if !strings.HasSuffix(out, " cleo-remote\n") {
		t.Errorf("expected trailing cleo-remote marker, got %q", out)
	}
	// The forced command must be the absolute path of the running binary.
	self, _ := os.Executable()
	self, _ = filepath.Abs(self)
	if !strings.Contains(out, `command="`+self+`"`) {
		t.Errorf("expected absolute self path %q in command=, got %q", self, out)
	}
}

func TestRemoteSetupPrintFromKeyFile(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "id_ed25519.pub")
	if err := os.WriteFile(keyFile, []byte(remoteTestPubKey+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runRemote(t, "", "remote", "setup", "--key", keyFile, "--print")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertManagedLine(t, out)
}

func TestRemoteSetupPrintFromStdin(t *testing.T) {
	out, _, err := runRemote(t, remoteTestPubKey+"\n", "remote", "setup", "--print")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertManagedLine(t, out)
}

// remoteHome redirects HOME to a temp dir so the write path never touches the
// developer's real ~/.ssh, and returns the authorized_keys path under it.
func remoteHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return filepath.Join(home, ".ssh", "authorized_keys")
}

func writeKeyFile(t *testing.T, key string) string {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "id_ed25519.pub")
	if err := os.WriteFile(keyFile, []byte(key+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return keyFile
}

func countManagedLines(content string) int {
	n := 0
	for _, l := range strings.Split(content, "\n") {
		if strings.HasSuffix(strings.TrimRight(l, " \t"), " cleo-remote") {
			n++
		}
	}
	return n
}

func TestRemoteSetupWritesAuthorizedKeys(t *testing.T) {
	akPath := remoteHome(t)
	keyFile := writeKeyFile(t, remoteTestPubKey)

	out, _, err := runRemote(t, "", "remote", "setup", "--key", keyFile)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	content, err := os.ReadFile(akPath)
	if err != nil {
		t.Fatalf("authorized_keys not written: %v", err)
	}
	if !strings.Contains(string(content), remoteTestKeyBody) || !strings.Contains(string(content), " cleo-remote") {
		t.Errorf("authorized_keys missing managed line: %q", content)
	}

	// ~/.ssh is 0700 and authorized_keys is 0600.
	if info, _ := os.Stat(filepath.Dir(akPath)); info.Mode().Perm() != 0o700 {
		t.Errorf("~/.ssh perm = %o, want 0700", info.Mode().Perm())
	}
	if info, _ := os.Stat(akPath); info.Mode().Perm() != 0o600 {
		t.Errorf("authorized_keys perm = %o, want 0600", info.Mode().Perm())
	}

	// Stdout reports success and the Termius next steps.
	if !strings.Contains(out, "Authorized") || !strings.Contains(out, "Termius") {
		t.Errorf("missing success/next-steps output: %q", out)
	}
}

func TestRemoteSetupIsIdempotent(t *testing.T) {
	akPath := remoteHome(t)
	keyFile := writeKeyFile(t, remoteTestPubKey)

	if _, _, err := runRemote(t, "", "remote", "setup", "--key", keyFile); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first, _ := os.ReadFile(akPath)

	out, _, err := runRemote(t, "", "remote", "setup", "--key", keyFile)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	second, _ := os.ReadFile(akPath)

	if string(first) != string(second) {
		t.Errorf("re-run changed the file:\n first: %q\nsecond: %q", first, second)
	}
	if n := countManagedLines(string(second)); n != 1 {
		t.Errorf("expected exactly 1 managed line after re-run, got %d", n)
	}
	if !strings.Contains(out, "no change") {
		t.Errorf("expected a no-change summary on the second run, got %q", out)
	}
}

func TestRemoteSetupPreservesOtherKeysAndReplacesInPlace(t *testing.T) {
	akPath := remoteHome(t)
	if err := os.MkdirAll(filepath.Dir(akPath), 0o700); err != nil {
		t.Fatal(err)
	}
	otherKey := "ssh-rsa AAAAB3NzaUNRELATED bob@laptop"
	staleManaged := `restrict,pty,command="/old/cleo" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITESTKEY cleo-remote`
	seed := otherKey + "\n" + staleManaged + "\n"
	if err := os.WriteFile(akPath, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	keyFile := writeKeyFile(t, remoteTestPubKey)
	if _, _, err := runRemote(t, "", "remote", "setup", "--key", keyFile); err != nil {
		t.Fatalf("execute: %v", err)
	}

	content, _ := os.ReadFile(akPath)
	if !strings.Contains(string(content), otherKey) {
		t.Errorf("unrelated key was dropped: %q", content)
	}
	if strings.Contains(string(content), "/old/cleo") {
		t.Errorf("stale managed line not replaced: %q", content)
	}
	if n := countManagedLines(string(content)); n != 1 {
		t.Errorf("expected exactly 1 managed line, got %d in %q", n, content)
	}
}

func TestRemoteSetupEmptyKeyErrors(t *testing.T) {
	_, _, err := runRemote(t, "   \n", "remote", "setup", "--print")
	if err == nil {
		t.Fatal("expected an error for an empty pasted key")
	}
}

func TestRemoteSetupMissingKeyFileErrors(t *testing.T) {
	_, _, err := runRemote(t, "", "remote", "setup", "--key", filepath.Join(t.TempDir(), "nope.pub"), "--print")
	if err == nil {
		t.Fatal("expected an error for a missing key file")
	}
}
