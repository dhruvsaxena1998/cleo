package cli

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/remote"
)

// newRemoteCmd returns the `cleo remote` group, which sets up phone/remote SSH
// access to the dashboard. getCtx is unused for now — remote setup edits
// ~/.ssh, not the cleo data dir — but the group keeps the registry signature
// uniform with the other command groups.
func newRemoteCmd(getCtx func() *Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Set up phone/remote SSH access to the dashboard",
	}
	cmd.AddCommand(newRemoteSetupCmd())
	return cmd
}

// newRemoteSetupCmd returns `cleo remote setup`. It authorizes a phone's SSH
// key to open the dashboard: by default it merges the managed forced-command
// line into ~/.ssh/authorized_keys (idempotently, preserving other keys);
// with --print it writes the computed line to stdout and touches no files.
func newRemoteSetupCmd() *cobra.Command {
	var keyPath string
	var printOnly bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Authorize a phone's SSH key to open the dashboard",
		Long: "Authorize a phone's SSH public key so connecting over SSH lands\n" +
			"straight in the Cleo dashboard. By default the managed line is merged\n" +
			"into ~/.ssh/authorized_keys (preserving your other keys); with --print\n" +
			"the line is written to stdout and no file is touched. Pass the key with\n" +
			"--key <path.pub>, or omit it to paste the key on stdin.",
		RunE: func(cmd *cobra.Command, args []string) error {
			pubKey, err := readPubKey(cmd, keyPath)
			if err != nil {
				return err
			}

			// Absolute path: SSH forced commands run without the login PATH,
			// so a bare "cleo" would not resolve.
			cleoBin, err := os.Executable()
			if err != nil {
				return err
			}
			cleoBin, _ = filepath.Abs(cleoBin)

			if printOnly {
				merged, err := remote.Merge("", pubKey, cleoBin)
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), merged)
				return nil
			}

			return writeAuthorizedKey(cmd.OutOrStdout(), pubKey, cleoBin)
		},
	}
	cmd.Flags().StringVar(&keyPath, "key", "", "path to the phone's SSH public key (.pub); omit to paste on stdin")
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the managed line to stdout without writing any file")
	return cmd
}

// readPubKey reads the mobile public key from the --key file when given, or
// from stdin as a paste fallback. The returned key is trimmed of surrounding
// whitespace; normalization to "<type> <base64>" happens in the pure module.
func readPubKey(cmd *cobra.Command, keyPath string) (string, error) {
	if keyPath != "" {
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return "", fmt.Errorf("read public key: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Paste the phone's SSH public key, then press Ctrl-D:")
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", errors.New("no public key provided: pass --key <path.pub> or paste a key on stdin")
	}
	return key, nil
}

// writeAuthorizedKey merges the managed line into ~/.ssh/authorized_keys via
// the pure module and persists it, ensuring ~/.ssh is 0700 and the file 0600.
// The write is atomic (temp file + rename) so an interrupted write can never
// truncate authorized_keys and lock the user out. An unchanged result is a
// no-op. It then warns (best-effort, non-privileged) if Remote Login does not
// appear to be running and prints the Termius next steps.
func writeAuthorizedKey(w io.Writer, pubKey, cleoBin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sshDir := filepath.Join(home, ".ssh")
	akPath := filepath.Join(sshDir, "authorized_keys")

	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", sshDir, err)
	}
	// Tighten perms in case ~/.ssh pre-existed with a looser mode.
	_ = os.Chmod(sshDir, 0o700)

	existing, err := readFileAllowMissing(akPath)
	if err != nil {
		return err
	}

	merged, err := remote.Merge(existing, pubKey, cleoBin)
	if err != nil {
		return err
	}

	changed := merged != existing
	if changed {
		if err := writeFileAtomic(akPath, []byte(merged), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", akPath, err)
		}
	}

	printSetupSummary(w, akPath, changed)
	if !sshdReachable() {
		printRemoteLoginWarning(w)
	}
	printTermiusNextSteps(w)
	return nil
}

// readFileAllowMissing reads a file, treating "does not exist" as empty content
// rather than an error — a first-time setup has no authorized_keys yet.
func readFileAllowMissing(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

// writeFileAtomic writes data to a temp file in the target's directory, then
// renames it over the destination. The rename is atomic within a filesystem,
// so a partial write never leaves a truncated authorized_keys behind.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".authorized_keys-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // harmless no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// sshdReachable is a best-effort, non-privileged probe for whether Remote Login
// (sshd) is accepting connections: it dials loopback:22 with a short timeout.
// It starts no process and never requests elevation — at worst it reports a
// false negative and the command prints an unnecessary warning.
func sshdReachable() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:22", 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func printSetupSummary(w io.Writer, akPath string, changed bool) {
	if changed {
		fmt.Fprintf(w, "%s Authorized the mobile key in %s\n",
			initOkStyle.Render("✓"), tildePath(akPath))
	} else {
		fmt.Fprintf(w, "%s Mobile key already authorized in %s (no change)\n",
			initOkStyle.Render("✓"), tildePath(akPath))
	}
}

// printRemoteLoginWarning explains how to turn on Remote Login. Cleo never
// enables it, never asks for sudo, and never starts a process — the user does
// this themselves.
func printRemoteLoginWarning(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, initWarnStyle.Render("⚠ Remote Login does not appear to be running"))
	fmt.Fprintln(w, "  Nothing can connect until sshd is accepting connections. Enable it yourself:")
	fmt.Fprintln(w, "    macOS: System Settings → General → Sharing → Remote Login")
	fmt.Fprintln(w, "  Cleo never enables this for you and never asks for sudo.")
}

func printTermiusNextSteps(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps (Termius on your phone):")
	fmt.Fprintln(w, "  1. Keychain → import the private key that matches this public key.")
	fmt.Fprintf(w, "  2. New Host → address = this Mac's LAN IP or hostname, username = %s.\n", currentUsername())
	fmt.Fprintln(w, "  3. Attach the key to the host and connect — you land straight in the dashboard.")
	fmt.Fprintln(w, "  Quitting the dashboard ends the command and closes the SSH session.")
}

// currentUsername returns the login name for the Termius host hint, falling
// back to $USER if the OS lookup is unavailable.
func currentUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	return "<your-mac-username>"
}
