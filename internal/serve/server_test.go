package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func testServer(t *testing.T, token string, sessions []state.Session) http.Handler {
	t.Helper()
	srv, err := NewServer(token, func() []state.Session { return sessions })
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv.Handler()
}

func TestSessionsEndpointReturnsProjectionWithValidToken(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ProjectID: "myapp", Agent: "claude", Name: "checkout", State: state.WaitingForInput, LastEventAt: now},
	}
	h := testServer(t, "sekret", sessions)

	req := httptest.NewRequest("GET", "/api/sessions?token=sekret", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	var v View
	if err := json.Unmarshal(rr.Body.Bytes(), &v); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, rr.Body.String())
	}
	if v.NeedCount != 1 || len(v.Projects) != 1 || v.Projects[0].Sessions[0].Name != "checkout" {
		t.Errorf("unexpected projection: %+v", v)
	}
}

func TestIndexServesPageWithInlinedIconsWhenAuthorized(t *testing.T) {
	h := testServer(t, "sekret", nil)

	req := httptest.NewRequest("GET", "/?token=sekret", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q, want text/html", ct)
	}
	body := rr.Body.String()
	if strings.Contains(body, iconPlaceholder) {
		t.Error("icon placeholder was not replaced")
	}
	if !strings.Contains(body, "D97757") { // claude brand colour, proves the SVG inlined
		t.Error("expected inlined claude SVG markup in the page")
	}
	if !strings.Contains(body, "cleo") {
		t.Error("expected cleo wordmark in the page")
	}
}

func TestIndexRejectsBadToken(t *testing.T) {
	h := testServer(t, "sekret", nil)
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rr.Code)
	}
}

func TestSessionsEndpointRejectsBadToken(t *testing.T) {
	sessions := []state.Session{
		{ProjectID: "myapp", Agent: "claude", Name: "checkout", State: state.Running, LastEventAt: time.Now()},
	}
	h := testServer(t, "sekret", sessions)

	for _, url := range []string{"/api/sessions", "/api/sessions?token=", "/api/sessions?token=wrong"} {
		req := httptest.NewRequest("GET", url, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", url, rr.Code)
		}
		if strings.Contains(rr.Body.String(), "checkout") {
			t.Errorf("%s: leaked session data to unauthorized caller: %s", url, rr.Body.String())
		}
	}
}
