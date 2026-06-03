package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

// newRemoteSetupCmd returns `cleo remote setup`. This slice implements
// --print: it computes the single managed forced-command authorized_keys line
// and writes it to stdout without touching the filesystem. The authorized_keys
// write lands in a later slice.
func newRemoteSetupCmd() *cobra.Command {
	var keyPath string
	var printOnly bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Authorize a phone's SSH key to open the dashboard",
		Long: "Compute the managed authorized_keys line that pins an incoming SSH\n" +
			"connection to the Cleo dashboard. Pass the phone's public key with\n" +
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

			merged, err := remote.Merge("", pubKey, cleoBin)
			if err != nil {
				return err
			}

			if !printOnly {
				return errors.New("writing ~/.ssh/authorized_keys is not implemented yet (see issue #94); re-run with --print to preview the managed line")
			}
			fmt.Fprint(cmd.OutOrStdout(), merged)
			return nil
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
