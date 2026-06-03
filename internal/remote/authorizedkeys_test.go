package remote

import (
	"strings"
	"testing"
)

const (
	testCleo = "/opt/homebrew/bin/cleo"
	testKey  = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITESTKEYBASE64"
)

// testKeyWithComment carries a trailing comment that the managed line must drop.
const testKeyWithComment = testKey + " phone@iphone"

// wantLine is the canonical managed line for testKey + testCleo.
const wantLine = `restrict,pty,command="/opt/homebrew/bin/cleo" ` + testKey + " " + ManagedComment

func TestManagedLine(t *testing.T) {
	tests := []struct {
		name    string
		pubKey  string
		cleo    string
		want    string
		wantErr bool
	}{
		{
			name:   "type and key only",
			pubKey: testKey,
			cleo:   testCleo,
			want:   wantLine,
		},
		{
			name:   "drops trailing key comment",
			pubKey: testKeyWithComment,
			cleo:   testCleo,
			want:   wantLine,
		},
		{
			name:   "collapses surrounding whitespace",
			pubKey: "  " + testKey + "   phone@iphone  ",
			cleo:   testCleo,
			want:   wantLine,
		},
		{
			name:   "escapes quotes and backslashes in path",
			pubKey: testKey,
			cleo:   `/weird/pa"th\cleo`,
			want:   `restrict,pty,command="/weird/pa\"th\\cleo" ` + testKey + " " + ManagedComment,
		},
		{
			name:    "rejects empty key",
			pubKey:  "",
			cleo:    testCleo,
			wantErr: true,
		},
		{
			name:    "rejects key with no base64 field",
			pubKey:  "ssh-ed25519",
			cleo:    testCleo,
			wantErr: true,
		},
		{
			name:    "rejects empty cleo path",
			pubKey:  testKey,
			cleo:    "   ",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ManagedLine(tc.pubKey, tc.cleo)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got line %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("ManagedLine\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	existingKey := "ssh-rsa AAAAB3NzaC1ycOTHERKEY alice@laptop"
	otherKey := "ssh-ed25519 AAAAC3NzaC1lYET another@host"
	oldManaged := `restrict,pty,command="/old/path/cleo" ssh-ed25519 AAAAC3NzaOLDKEY ` + ManagedComment

	tests := []struct {
		name     string
		existing string
		pubKey   string
		cleo     string
		want     string
		wantErr  bool
	}{
		{
			name:     "empty content appends the managed line",
			existing: "",
			pubKey:   testKey,
			cleo:     testCleo,
			want:     wantLine + "\n",
		},
		{
			name:     "whitespace-only content treated as empty (no newline)",
			existing: "",
			pubKey:   testKeyWithComment,
			cleo:     testCleo,
			want:     wantLine + "\n",
		},
		{
			name:     "appends after an existing unrelated key",
			existing: existingKey + "\n",
			pubKey:   testKey,
			cleo:     testCleo,
			want:     existingKey + "\n" + wantLine + "\n",
		},
		{
			name:     "appends when existing content lacks a trailing newline",
			existing: existingKey,
			pubKey:   testKey,
			cleo:     testCleo,
			want:     existingKey + "\n" + wantLine + "\n",
		},
		{
			name:     "idempotent: re-merging identical inputs is a no-op",
			existing: existingKey + "\n" + wantLine + "\n",
			pubKey:   testKey,
			cleo:     testCleo,
			want:     existingKey + "\n" + wantLine + "\n",
		},
		{
			name:     "replaces a changed key in place, preserving position",
			existing: existingKey + "\n" + oldManaged + "\n" + otherKey + "\n",
			pubKey:   testKey,
			cleo:     "/old/path/cleo",
			want:     existingKey + "\n" + `restrict,pty,command="/old/path/cleo" ` + testKey + " " + ManagedComment + "\n" + otherKey + "\n",
		},
		{
			name:     "replaces a changed path in place, same key",
			existing: existingKey + "\n" + oldManaged + "\n",
			pubKey:   "ssh-ed25519 AAAAC3NzaOLDKEY",
			cleo:     testCleo,
			want:     existingKey + "\n" + `restrict,pty,command="/opt/homebrew/bin/cleo" ssh-ed25519 AAAAC3NzaOLDKEY ` + ManagedComment + "\n",
		},
		{
			name:     "preserves blank lines and comments around the managed line",
			existing: "# my keys\n\n" + existingKey + "\n",
			pubKey:   testKey,
			cleo:     testCleo,
			want:     "# my keys\n\n" + existingKey + "\n" + wantLine + "\n",
		},
		{
			name:     "collapses multiple managed lines into one in place",
			existing: oldManaged + "\n" + existingKey + "\n" + oldManaged + "\n",
			pubKey:   testKey,
			cleo:     testCleo,
			want:     wantLine + "\n" + existingKey + "\n",
		},
		{
			name:    "propagates an invalid key error",
			pubKey:  "bogus",
			cleo:    testCleo,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Merge(tc.existing, tc.pubKey, tc.cleo)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Merge\n got: %q\nwant: %q", got, tc.want)
			}
			// Every successful merge result ends in exactly one newline and
			// contains exactly one managed line.
			if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n\n") {
				t.Errorf("result trailing newline malformed: %q", got)
			}
			if n := countManaged(got); n != 1 {
				t.Errorf("expected exactly 1 managed line, found %d in %q", n, got)
			}
		})
	}
}

func countManaged(content string) int {
	n := 0
	for _, l := range strings.Split(content, "\n") {
		if isManaged(l) {
			n++
		}
	}
	return n
}
