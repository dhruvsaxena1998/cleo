package config

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
)

// Keymap holds the resolved main-view key bindings. It is computed once during
// Load() from the [keybinds] table and consumed read-only by the TUI.
type Keymap struct {
	Up, Down, Enter, New, View, Kill, Prune, Rename, Remove, Editor, Filter, Send, Mute, Help, Quit, Esc, Space key.Binding
}

// keybindAction describes a single rebindable action. The slice order is the
// canonical importance ordering: it triples as the validation namespace, the
// help-screen order, and the conflict-precedence ranking.
type keybindAction struct {
	name        string
	defaultKeys []string
	desc        string
	field       func(*Keymap) *key.Binding
}

var keybindActions = []keybindAction{
	{"up", []string{"up", "k"}, "up", func(m *Keymap) *key.Binding { return &m.Up }},
	{"down", []string{"down", "j"}, "down", func(m *Keymap) *key.Binding { return &m.Down }},
	{"attach", []string{"enter"}, "attach", func(m *Keymap) *key.Binding { return &m.Enter }},
	{"expand", []string{" "}, "expand / collapse", func(m *Keymap) *key.Binding { return &m.Space }},
	{"find", []string{"/"}, "find", func(m *Keymap) *key.Binding { return &m.Filter }},
	{"new", []string{"n"}, "new", func(m *Keymap) *key.Binding { return &m.New }},
	{"view", []string{"v"}, "view", func(m *Keymap) *key.Binding { return &m.View }},
	{"send", []string{"m"}, "send", func(m *Keymap) *key.Binding { return &m.Send }},
	{"editor", []string{"ctrl+g", "e"}, "open editor", func(m *Keymap) *key.Binding { return &m.Editor }},
	{"rename", []string{"r"}, "rename", func(m *Keymap) *key.Binding { return &m.Rename }},
	{"kill", []string{"K", "ctrl+k"}, "kill", func(m *Keymap) *key.Binding { return &m.Kill }},
	{"prune", []string{"P"}, "prune finished", func(m *Keymap) *key.Binding { return &m.Prune }},
	{"remove", []string{"D"}, "remove project", func(m *Keymap) *key.Binding { return &m.Remove }},
	{"mute", []string{"alt+m"}, "mute", func(m *Keymap) *key.Binding { return &m.Mute }},
	{"help", []string{"?"}, "help", func(m *Keymap) *key.Binding { return &m.Help }},
	{"quit", []string{"q"}, "quit", func(m *Keymap) *key.Binding { return &m.Quit }},
	{"close", []string{"esc"}, "close", func(m *Keymap) *key.Binding { return &m.Esc }},
}

// specialKeys is the best-effort allowlist of non-printable key names bubbletea
// emits. Combined with single printable runes and the ctrl+/alt+ prefixes it
// forms the validation namespace; anything else is a typo and gets dropped.
var specialKeys = map[string]bool{
	"up": true, "down": true, "left": true, "right": true,
	"enter": true, "esc": true, "tab": true, "shift+tab": true,
	"backspace": true, "delete": true, "home": true, "end": true,
	"pgup": true, "pgdown": true,
}

// validKey reports whether k is a key string the TUI could actually receive:
// a known special key, a single printable rune, or a ctrl+/alt+ modified rune.
func validKey(k string) bool {
	if specialKeys[k] {
		return true
	}
	if r := []rune(k); len(r) == 1 && unicode.IsPrint(r[0]) {
		return true
	}
	for _, prefix := range []string{"ctrl+", "alt+"} {
		if rest, ok := strings.CutPrefix(k, prefix); ok {
			r := []rune(rest)
			return len(r) == 1 && unicode.IsPrint(r[0])
		}
	}
	return false
}

// reservedKeys maps each always-on hatch key to the action that legitimately
// owns it (or "" for ctrl+c, the hardwired quit hatch that no action may
// claim). The TUI honors these keys regardless of config; here we stop any
// *other* action from binding them so the hatches can never be shadowed.
var reservedKeys = map[string]string{
	"enter":  "attach",
	"esc":    "close",
	"ctrl+c": "",
}

// isReservedFrom reports whether key k is a reserved hatch that action may not
// claim — true for any action other than the key's legitimate owner.
func isReservedFrom(k, action string) bool {
	owner, ok := reservedKeys[k]
	return ok && owner != action
}

// resolveKeymap builds the Keymap from user overrides and returns any problems
// as warnings. Per-action replace: listing an action replaces its keys
// entirely; an omitted action keeps its default; an empty list reverts to
// default (no disabled state). Each key is validated individually, so an
// invalid key is dropped while the rest of the action's list survives; an
// action left with no valid keys reverts to its default to preserve the
// "every action has ≥1 key" invariant.
func resolveKeymap(binds map[string][]string) (Keymap, []Diagnostic) {
	var km Keymap
	var diags []Diagnostic
	claimed := map[string]string{} // key -> name of the action that owns it
	wonShown := map[string]bool{}  // key -> already emitted a ✓ for its winner

	// claim filters one action's requested keys, taking the valid+unclaimed
	// ones. When record is true it also notes why the rest were dropped; the
	// silent fallback pass below passes record=false so a recovered default is
	// not reported as a fresh conflict on top of the override that already was.
	claim := func(action string, keys []string, record bool) []string {
		var out []string
		for _, k := range keys {
			switch {
			case !validKey(k):
				if record {
					diags = append(diags, Diagnostic{OK: false, Detail: fmt.Sprintf("keybinds.%s: %q is not a recognized key; ignored", action, k)})
				}
			case isReservedFrom(k, action):
				if record {
					diags = append(diags, Diagnostic{OK: false, Detail: fmt.Sprintf("keybinds.%s: %q is reserved and cannot be rebound; ignored", action, k)})
				}
			case claimed[k] != "" && claimed[k] != action:
				if record {
					winner := claimed[k]
					diags = append(diags, Diagnostic{OK: false, Detail: fmt.Sprintf("keybinds.%s: %q is already bound to %q; ignored", action, k, winner)})
					if !wonShown[k] {
						diags = append(diags, Diagnostic{OK: true, Detail: fmt.Sprintf("keybinds.%s: %q is active", winner, k)})
						wonShown[k] = true
					}
				}
			default:
				claimed[k] = action
				out = append(out, k)
			}
		}
		return out
	}

	for _, a := range keybindActions {
		keys := a.defaultKeys
		overridden := false
		if override, ok := binds[a.name]; ok && len(override) > 0 {
			keys = override
			overridden = true
		}
		valid := claim(a.name, keys, true)
		if len(valid) == 0 && overridden {
			// The user's keys were all dropped; quietly fall back to this
			// action's defaults so a typo'd override never silently disables it.
			valid = claim(a.name, a.defaultKeys, false)
		}
		helpKey := ""
		if len(valid) > 0 {
			helpKey = valid[0]
		}
		*a.field(&km) = key.NewBinding(
			key.WithKeys(valid...),
			key.WithHelp(helpKey, a.desc),
		)
	}
	return km, diags
}
