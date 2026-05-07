package tui

func (m Model) View() string {
	// Composed in tasks below: header, sidebar, main pane, footer.
	return renderFrame(m)
}
