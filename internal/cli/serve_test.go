package cli

import "testing"

func TestServeCmdWiring(t *testing.T) {
	cmd := newServeCmd(func() *Ctx { return nil })
	if cmd.Use != "serve" {
		t.Errorf("Use = %q, want serve", cmd.Use)
	}
	f := cmd.Flags().Lookup("port")
	if f == nil {
		t.Fatal("missing --port flag")
	}
	if f.DefValue != "7777" {
		t.Errorf("--port default = %q, want 7777", f.DefValue)
	}
}

func TestServeCmdRegistered(t *testing.T) {
	root := NewRootCmd(func(*Ctx) error { return nil })
	for _, c := range root.Commands() {
		if c.Name() == "serve" {
			return
		}
	}
	t.Error("serve command not registered on root")
}
