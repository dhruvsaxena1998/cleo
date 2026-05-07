package projects

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	store := NewStore(path)

	p, err := store.Add("/Users/x/Dev/myapp")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "myapp" {
		t.Errorf("id %q", p.ID)
	}

	got, err := store.Get("myapp")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/Users/x/Dev/myapp" {
		t.Errorf("path %q", got.Path)
	}
}

func TestAddDuplicateIDDeconflicts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	store := NewStore(path)
	_, _ = store.Add("/foo/myapp")
	p2, err := store.Add("/bar/myapp")
	if err != nil {
		t.Fatal(err)
	}
	if p2.ID != "myapp-2" {
		t.Errorf("expected myapp-2, got %q", p2.ID)
	}
}

func TestRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	store := NewStore(path)
	_, _ = store.Add("/foo/myapp")
	if err := store.Remove("myapp"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("myapp"); err == nil {
		t.Errorf("expected ErrNotFound")
	}
}

func TestList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	store := NewStore(path)
	_, _ = store.Add("/foo/a")
	_, _ = store.Add("/foo/b")
	got, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("len %d", len(got))
	}
}

func TestResolveFromCwd(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "projects.json"))
	root := filepath.Join(dir, "myapp")
	if err := os.MkdirAll(filepath.Join(root, "src", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(root); err != nil {
		t.Fatal(err)
	}
	p, err := store.ResolveFromCwd(filepath.Join(root, "src", "deep"))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "myapp" {
		t.Errorf("got %q", p.ID)
	}
	if _, err := store.ResolveFromCwd(t.TempDir()); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
