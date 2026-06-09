package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The frame core is a pure function — spec in, rendered string out — so its
// interface is the test surface. These table-driven cases pin the structural
// invariants every popup and panel relies on, using plain (unstyled) Border and
// Fill so widths measure cleanly. Per-popup byte-for-byte behaviour is pinned by
// each popup's own view tests.

func plainSpec(s frameSpec) frameSpec {
	s.Border = lipgloss.NewStyle()
	s.Fill = lipgloss.NewStyle()
	return s
}

func frameLines(spec frameSpec) []string {
	return strings.Split(drawFrame(spec), "\n")
}

func TestFrameEveryLineMatchesWidth(t *testing.T) {
	specs := map[string]frameSpec{
		"content-sized": plainSpec(frameSpec{Width: 40, Title: "Title", Hint: "hint", Sections: [][]string{{"a", "b"}}}),
		"two-section":   plainSpec(frameSpec{Width: 50, Title: "T", Hint: "h", Sections: [][]string{{"body"}, {"footer"}}}),
		"fixed-height":  plainSpec(frameSpec{Width: 30, Title: "P", Hint: "", Sections: [][]string{{"x"}}, Height: 10}),
		"title-border":  plainSpec(frameSpec{Width: 60, Title: "Quick", Hint: "id", Sections: [][]string{{"row"}}, TitleInBorder: true}),
		"narrow-clamp":  plainSpec(frameSpec{Width: 5, Title: "X", Hint: "y", Sections: [][]string{{"z"}}}),
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			want := spec.Width
			if want-2 < 4 { // drawFrame clamps the inner width to 4 → outer 6
				want = 6
			}
			for i, line := range frameLines(spec) {
				if got := lipgloss.Width(line); got != want {
					t.Errorf("line %d width = %d, want %d: %q", i, got, want, ansi.Strip(line))
				}
			}
		})
	}
}

func TestFrameBorderGlyphs(t *testing.T) {
	lines := frameLines(plainSpec(frameSpec{Width: 30, Title: "T", Hint: "h", Sections: [][]string{{"a"}}}))
	top, bottom := lines[0], lines[len(lines)-1]
	if !strings.HasPrefix(top, "┌") || !strings.HasSuffix(top, "┐") {
		t.Errorf("top border = %q, want ┌…┐", top)
	}
	if !strings.HasPrefix(bottom, "└") || !strings.HasSuffix(bottom, "┘") {
		t.Errorf("bottom border = %q, want └…┘", bottom)
	}
}

func TestFrameDividerCountIsSectionsPlusTitle(t *testing.T) {
	cases := []struct {
		name          string
		sections      [][]string
		titleInBorder bool
		want          int // ├ dividers expected
	}{
		{"one section + title", [][]string{{"a"}}, false, 1},         // title divider only
		{"two sections + title", [][]string{{"a"}, {"b"}}, false, 2}, // title + one between
		{"three sections + title", [][]string{{"a"}, {"b"}, {"c"}}, false, 3},
		{"one section, title in border", [][]string{{"a"}}, true, 0}, // no title divider, no between
		{"two sections, title in border", [][]string{{"a"}, {"b"}}, true, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := drawFrame(plainSpec(frameSpec{Width: 30, Title: "T", Hint: "h", Sections: tc.sections, TitleInBorder: tc.titleInBorder}))
			if got := strings.Count(out, "├"); got != tc.want {
				t.Errorf("divider count = %d, want %d\n%s", got, tc.want, out)
			}
		})
	}
}

func TestFrameHintIsRightAligned(t *testing.T) {
	lines := frameLines(plainSpec(frameSpec{Width: 40, Title: "Left", Hint: "Right", Sections: [][]string{{"x"}}}))
	// Strip the border glyphs and flanking pad to get just the title content.
	titleRow := strings.Trim(ansi.Strip(lines[1]), "│ ")
	if !strings.HasSuffix(titleRow, "Right") {
		t.Errorf("hint should be right-aligned in the title row, got %q", titleRow)
	}
	if !strings.HasPrefix(titleRow, "Left") {
		t.Errorf("title should be present on the left, got %q", titleRow)
	}
	// The gap between title and hint should be filled with spaces so the row is
	// exactly the inner width.
	if strings.Contains(strings.TrimSpace(titleRow), "Left Right") {
		t.Errorf("title and hint should be gap-filled apart, got %q", titleRow)
	}
}

func TestFrameTruncatesWideRowsAndPadsNarrow(t *testing.T) {
	wide := strings.Repeat("x", 200)
	lines := frameLines(plainSpec(frameSpec{Width: 30, Title: "T", Hint: "", Sections: [][]string{{wide, "short"}}}))
	// Inner content width is Width-4 = 26; every body row must fit exactly.
	for _, idx := range []int{3, 4} { // body rows sit after top, title, divider
		body := ansi.Strip(lines[idx])
		if lipgloss.Width(lines[idx]) != 30 {
			t.Fatalf("body line %d width = %d, want 30: %q", idx, lipgloss.Width(lines[idx]), body)
		}
	}
	// The wide row must have been truncated with an ellipsis (no overflow).
	if !strings.Contains(ansi.Strip(lines[3]), "…") {
		t.Errorf("wide row should be truncated with ellipsis, got %q", ansi.Strip(lines[3]))
	}
}

func TestFrameHeightZeroSizesToContent(t *testing.T) {
	lines := frameLines(plainSpec(frameSpec{Width: 30, Title: "T", Hint: "", Sections: [][]string{{"a", "b", "c"}}}))
	// top + title + divider + 3 body + bottom = 7
	if got := len(lines); got != 7 {
		t.Errorf("content-sized height = %d lines, want 7\n%s", got, strings.Join(lines, "\n"))
	}
}

func TestFrameFixedHeightPadsAndTruncates(t *testing.T) {
	t.Run("pads short body with blank rows", func(t *testing.T) {
		lines := frameLines(plainSpec(frameSpec{Width: 30, Title: "T", Hint: "", Sections: [][]string{{"only"}}, Height: 10}))
		if got := len(lines); got != 10 {
			t.Fatalf("fixed height = %d lines, want 10", got)
		}
	})
	t.Run("truncates long body to fit", func(t *testing.T) {
		body := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
		lines := frameLines(plainSpec(frameSpec{Width: 30, Title: "T", Hint: "", Sections: [][]string{body}, Height: 7}))
		if got := len(lines); got != 7 {
			t.Fatalf("fixed height = %d lines, want 7", got)
		}
		// Body budget is 7-4 = 3 rows, so only the first three survive.
		out := ansi.Strip(strings.Join(lines, "\n"))
		if !strings.Contains(out, "3") || strings.Contains(out, "4") {
			t.Errorf("expected first 3 body rows only, got\n%s", out)
		}
	})
}

func TestFrameTitleInBorderHasNoSeparateTitleRow(t *testing.T) {
	withBorder := frameLines(plainSpec(frameSpec{Width: 60, Title: "Quick Message", Hint: "cleo-x", Sections: [][]string{{"a", "b"}}, TitleInBorder: true}))
	// Title baked into the top line itself.
	if !strings.Contains(ansi.Strip(withBorder[0]), "Quick Message") {
		t.Errorf("title-in-border top line should carry the title, got %q", ansi.Strip(withBorder[0]))
	}
	// top + 2 body + bottom = 4 lines, no title row and no title divider.
	if got := len(withBorder); got != 4 {
		t.Errorf("title-in-border frame = %d lines, want 4\n%s", got, strings.Join(withBorder, "\n"))
	}
	if strings.Count(strings.Join(withBorder, "\n"), "├") != 0 {
		t.Errorf("title-in-border single section should have no dividers")
	}
}
