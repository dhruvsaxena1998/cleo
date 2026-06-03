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

func TestRemoteSetupWithoutPrintIsNotImplemented(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "id_ed25519.pub")
	if err := os.WriteFile(keyFile, []byte(remoteTestPubKey+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runRemote(t, "", "remote", "setup", "--key", keyFile)
	if err == nil {
		t.Fatal("expected an error when --print is omitted (write not yet implemented)")
	}
	if !strings.Contains(err.Error(), "--print") {
		t.Errorf("error should point the user at --print, got %q", err.Error())
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
