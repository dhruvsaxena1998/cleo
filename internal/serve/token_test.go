package serve

import "testing"

func TestURLBuildsTokenizedAddress(t *testing.T) {
	got := URL("192.168.1.5", 7777, "abc123")
	want := "http://192.168.1.5:7777/?token=abc123"
	if got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}

func TestTokenValidatesAgainstItself(t *testing.T) {
	tok, err := NewToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	if tok == "" {
		t.Fatal("newToken returned empty string")
	}
	if !tokenOK(tok, tok) {
		t.Error("a token should validate against itself")
	}
	if tokenOK(tok, "not-the-token") {
		t.Error("a wrong token must not validate")
	}
}
