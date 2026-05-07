package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

func testRootedCtx(t *testing.T, root string) func() *Ctx {
	t.Helper()
	return func() *Ctx {
		c, err := NewCtxWithRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
}

func mkdir(p string) error { return os.MkdirAll(p, 0o755) }

type fakeTmux struct {
	created []tmux.NewSessionOpts
	exists  map[string]bool
}

func (f *fakeTmux) NewSession(o tmux.NewSessionOpts) error {
	f.created = append(f.created, o)
	if f.exists == nil {
		f.exists = map[string]bool{}
	}
	f.exists[o.Name] = true
	return nil
}
func (f *fakeTmux) HasSession(n string) (bool, error) { return f.exists[n], nil }
func (f *fakeTmux) LsPrefix(p string) ([]string, error) {
	var out []string
	for k := range f.exists {
		if strings.HasPrefix(k, p) {
			out = append(out, k)
		}
	}
	return out, nil
}
func (f *fakeTmux) Kill(n string) error                     { delete(f.exists, n); return nil }
func (f *fakeTmux) CapturePane(string, int) (string, error) { return "", nil }
func (f *fakeTmux) RenameSession(from, to string) error     { return nil }
