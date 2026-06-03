package serve

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

//go:embed assets
var assetsFS embed.FS

// iconPlaceholder in index.html is replaced at startup with a JS object literal
// mapping agent name -> inline SVG markup, so the brand glyphs ship in the
// binary and currentColor-based marks pick up the page's colour.
const iconPlaceholder = "__AGENT_ICONS__"

var agentNames = []string{"claude", "codex", "opencode", "pi"}

// Server is the read-only remote view's HTTP surface (ADR 0004). It never
// attaches to or mutates a Session; it only projects the live session list.
type Server struct {
	token    string
	snapshot func() []state.Session
	now      func() time.Time
	page     []byte
}

// NewServer builds the server. snapshot returns the live session list (the CLI
// wires it to reconcile + state.List); token gates every request.
func NewServer(token string, snapshot func() []state.Session) (*Server, error) {
	page, err := buildPage()
	if err != nil {
		return nil, err
	}
	return &Server{
		token:    token,
		snapshot: snapshot,
		now:      time.Now,
		page:     page,
	}, nil
}

// Handler returns the routed, token-gated handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sessions", s.gate(s.handleSessions))
	mux.HandleFunc("/", s.gate(s.handleIndex))
	return mux
}

// gate rejects any request whose ?token= does not match, in constant time,
// before the wrapped handler runs. This is the whole access control: no token,
// no data.
func (s *Server) gate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !tokenOK(s.token, r.URL.Query().Get("token")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	view := Project(s.snapshot(), s.now())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(view)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(s.page)
}

// buildPage reads the embedded index.html and inlines the agent SVGs.
func buildPage() ([]byte, error) {
	html, err := assetsFS.ReadFile("assets/index.html")
	if err != nil {
		return nil, err
	}
	icons, err := iconsJS()
	if err != nil {
		return nil, err
	}
	if !strings.Contains(string(html), iconPlaceholder) {
		return nil, fmt.Errorf("index.html missing %s placeholder", iconPlaceholder)
	}
	return []byte(strings.Replace(string(html), iconPlaceholder, icons, 1)), nil
}

// iconsJS marshals the embedded brand SVGs into a JS object literal.
func iconsJS() (string, error) {
	m := map[string]string{}
	for _, n := range agentNames {
		b, err := assetsFS.ReadFile("assets/agent-" + n + ".svg")
		if err != nil {
			return "", err
		}
		m[n] = strings.TrimSpace(string(b))
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
