package main

import (
	tea "charm.land/bubbletea/v2"
)

// DatabaseState holds all state for the Database tab.
// Populated in Task 7.
type DatabaseState struct{}

func databaseView(m model) string {
	return styleDim.Render("\n  Database tab coming soon.\n")
}

func databaseUpdate(msg tea.Msg, m model) (model, tea.Cmd) {
	return m, nil
}
