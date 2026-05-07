package tui

import "github.com/charmbracelet/bubbles/key"

type Keymap struct {
	Up, Down, Left, Right, Enter, New, View, Kill, Rename, Add, Filter, Mute, Help, Quit, Esc key.Binding
}

func DefaultKeymap() Keymap {
	return Keymap{
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "attach")),
		New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		View:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view")),
		Kill:   key.NewBinding(key.WithKeys("K", "ctrl+k"), key.WithHelp("K", "kill")),
		Rename: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
		Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add project")),
		Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Mute:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mute")),
		Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:   key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Esc:    key.NewBinding(key.WithKeys("esc")),
	}
}
