package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/muesli/termenv"
)

func withTrueColor(t *testing.T) {
	t.Helper()
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })
}

// TestViewPaintsEveryCell protects terminals whose configured background is
// transparent. It asserts against the final View output, including popup
// composition, using x/ansi to decode cells independently of the painter.
func TestViewPaintsEveryCell(t *testing.T) {
	withTrueColor(t)

	for _, width := range []int{60, 100} {
		for _, popup := range []bool{false, true} {
			name := fmt.Sprintf("width-%d/popup-%t", width, popup)
			t.Run(name, func(t *testing.T) {
				m := backgroundTestModel(t, width, 30)
				if popup {
					m.mode = ModePopup
					m.popup = NewHelpPopup(m.theme, m.ctx.Config.Keymap, "")
				}
				assertCellsHaveBackground(t, m.View(), "")
				for i, line := range strings.Split(m.View(), "\n") {
					if got := lipgloss.Width(line); got != width {
						t.Fatalf("line %d width = %d, want %d", i, got, width)
					}
				}
			})
		}
	}
}

func backgroundTestModel(t *testing.T, width, height int) Model {
	t.Helper()
	c := newTestCtx(t)
	m := mkSessionModel(c)
	m.width, m.height = width, height
	m.sessions[0].Agent = "codex"
	m.sessions[0].StartedAt = time.Now().Add(-time.Minute)
	m.sessions[0].LastEventAt = time.Now()
	m.sessions[0].ToolCount = 2
	m.paneCache["s1"] = "\x1b[38;2;137;180;250mstyled preview\x1b[0m after reset"
	if err := c.Events("s1").Append(events.Entry{Type: "pre_tool_use", Tool: "Bash", Detail: "make test"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Events("s1").Append(events.Entry{Type: "notification", Detail: "review ready"}); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestHighlightedEventRowKeepsSurf0Background(t *testing.T) {
	withTrueColor(t)
	theme := Resolve("catppuccin-mocha")
	row := theme.FormatEventRow(events.Entry{Type: "notification", Detail: "review ready"}, 60, true)
	assertCellsHaveBackground(t, row, "48;2;48;50;68")
}

func TestBackgroundAfterSGR(t *testing.T) {
	tests := []struct {
		name   string
		params string
		before bool
		want   bool
	}{
		{"empty reset", "", true, false},
		{"explicit reset", "0", true, false},
		{"background reset", "49", true, false},
		{"ansi background", "44", false, true},
		{"bright background", "104", false, true},
		{"indexed foreground", "38;5;48", false, false},
		{"indexed background", "48;5;236", false, true},
		{"rgb foreground", "38;2;48;49;50", false, false},
		{"rgb background", "48;2;10;20;30", false, true},
		{"colon rgb foreground", "38:2::48:49:50", false, false},
		{"colon rgb background", "48:2::10:20:30", false, true},
		{"bare foreground", "38", true, true},
		{"bare background", "48", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backgroundAfterSGR(tt.params, tt.before); got != tt.want {
				t.Fatalf("backgroundAfterSGR(%q, %t) = %t, want %t", tt.params, tt.before, got, tt.want)
			}
		})
	}
}

// assertCellsHaveBackground uses x/ansi for sequence boundaries and its own
// deliberately narrow SGR model. expected is empty for any active background,
// or an exact extended-colour parameter prefix such as "48;2;49;50;68".
func assertCellsHaveBackground(t *testing.T, rendered, expected string) {
	t.Helper()
	var state byte
	active := ""
	line, column := 1, 0
	for len(rendered) > 0 {
		seq, width, n, nextState := ansi.DecodeSequence(rendered, state, nil)
		if n == 0 {
			t.Fatal("ANSI decoder made no progress")
		}
		state = nextState
		rendered = rendered[n:]
		if width == 0 {
			if strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "m") {
				active = backgroundFromSGR(seq[2:len(seq)-1], active)
			}
			if seq == "\n" {
				line++
				column = 0
			}
			continue
		}
		column += width
		if active == "" {
			t.Fatalf("cell at line %d, column %d has no explicit background", line, column)
		}
		if expected != "" && active != expected {
			t.Fatalf("cell at line %d, column %d background = %q, want %q", line, column, active, expected)
		}
	}
}

func backgroundFromSGR(params, active string) string {
	if params == "" {
		return ""
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		p, _ := strconv.Atoi(parts[i])
		switch {
		case p == 0 || p == 49:
			active = ""
		case p >= 40 && p <= 47 || p >= 100 && p <= 107:
			active = parts[i]
		case p == 38 && i+1 < len(parts):
			if parts[i+1] == "5" {
				i += 2
			} else if parts[i+1] == "2" {
				i += 4
			}
		case p == 48:
			end := i
			if i+1 < len(parts) && parts[i+1] == "5" {
				end = min(i+2, len(parts)-1)
			} else if i+1 < len(parts) && parts[i+1] == "2" {
				end = min(i+4, len(parts)-1)
			}
			active = strings.Join(parts[i:end+1], ";")
			i = end
		}
	}
	return active
}
