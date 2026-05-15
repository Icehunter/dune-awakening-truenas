package main

import (
	tea "charm.land/bubbletea/v2"
)

// BattlegroupState holds all state for the Battlegroup tab.
// Populated in Task 5.
type BattlegroupState struct{}

func battlegroupView(m model) string {
	return styleDim.Render("\n  Battlegroup tab coming soon.\n")
}

func battlegroupUpdate(msg tea.Msg, m model) (model, tea.Cmd) {
	return m, nil
}
