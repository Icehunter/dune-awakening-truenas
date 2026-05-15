package main

import (
	tea "charm.land/bubbletea/v2"
)

// LogsState holds all state for the Logs tab.
// Populated in Task 8.
type LogsState struct{}

func logsView(m model) string {
	return styleDim.Render("\n  Logs tab coming soon.\n")
}

func logsUpdate(msg tea.Msg, m model) (model, tea.Cmd) {
	return m, nil
}
