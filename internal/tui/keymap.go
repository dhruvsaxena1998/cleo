package tui

import "github.com/charmbracelet/bubbles/key"

type Keymap struct {
	Up, Down, Enter, New, View, Kill, Prune, Rename, Remove, Filter, Send, Mute, Help, Quit, Esc, Space key.Binding
}

func DefaultKeymap() Keymap {
	return Keymap{
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "attach")),
		New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		View:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view")),
		Kill:   key.NewBinding(key.WithKeys("K", "ctrl+k"), key.WithHelp("K", "kill")),
		Prune:  key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "prune finished")),
		Rename: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
		Remove: key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "remove project")),
		Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Send:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "send")),
		Mute:   key.NewBinding(key.WithKeys("alt+m"), key.WithHelp("alt+m", "mute")),
		Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:   key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Esc:    key.NewBinding(key.WithKeys("esc")),
		Space:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "expand / collapse")),
	}
}
