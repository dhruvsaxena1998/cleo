package ids

import "testing"

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"fix auth bug":     "fix-auth-bug",
		"FIX-Auth_BUG":     "fix-auth-bug",
		"  multi   space ": "multi-space",
		"foo!@#bar":        "foo-bar",
		"":                 "",
		"--leading":        "leading",
		"trailing--":       "trailing",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDedupeSlug(t *testing.T) {
	existing := map[string]bool{"foo": true, "foo-2": true, "bar": true}
	for in, want := range map[string]string{
		"baz": "baz",
		"foo": "foo-3",
		"bar": "bar-2",
	} {
		if got := DedupeSlug(in, existing); got != want {
			t.Errorf("DedupeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMakeSessionID(t *testing.T) {
	got := MakeSessionID("myapp", "claude", "fix-auth-bug")
	if got != "cleo-myapp-claude-fix-auth-bug" {
		t.Errorf("got %q", got)
	}
}
