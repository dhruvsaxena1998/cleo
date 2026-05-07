package cli

import (
	"os"
	"testing"
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
