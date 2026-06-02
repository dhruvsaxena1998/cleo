package config

import "github.com/charmbracelet/bubbles/key"

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

// resolveKeymap builds the Keymap from user overrides. Per-action replace:
// listing an action replaces its keys entirely; an omitted action keeps its
// default; an empty list reverts to default (no disabled state).
func resolveKeymap(binds map[string][]string) Keymap {
	var km Keymap
	for _, a := range keybindActions {
		keys := a.defaultKeys
		if override, ok := binds[a.name]; ok && len(override) > 0 {
			keys = override
		}
		*a.field(&km) = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(keys[0], a.desc),
		)
	}
	return km
}
