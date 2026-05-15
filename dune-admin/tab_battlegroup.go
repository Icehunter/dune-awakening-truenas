package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type bgSubView int

const (
	bgvMenu bgSubView = iota
	bgvStatus
	bgvStart
	bgvStop
	bgvRestart
	bgvUpdate
	bgvBackup
	bgvRestore
	bgvPods
)

// BattlegroupState holds all state for the Battlegroup tab.
type BattlegroupState struct {
	menu        int
	subView     bgSubView
	confirm     bool
	result      string
	loading     bool
	pods        []string
	selectedPod int
	shellCmd    string
}

var bgMenuLabels = []string{
	"Status", "Start", "Stop", "Restart", "Update",
	"Backup DB", "Restore DB", "Pods",
}

// bgScript is the path of the on-VM battlegroup management script.
const bgScript = "sudo ~/.dune/download/scripts/battlegroup.sh"

// Message types for the Battlegroup tab.
type msgBGStatus struct {
	output string
	err    error
}

type msgBGExec struct {
	output string
	err    error
}

type msgBGPods struct {
	lines []string
	err   error
}

// Commands

func cmdBGStatus() tea.Msg {
	out, err := sshExec(fmt.Sprintf(
		"sudo kubectl get pods -n %s -o wide 2>&1", globalPodNS))
	return msgBGStatus{output: out, err: err}
}

func cmdBGExec(subCmd string) tea.Cmd {
	return func() tea.Msg {
		out, err := sshExec(bgScript + " " + subCmd)
		return msgBGExec{output: out, err: err}
	}
}

func cmdBGPods() tea.Msg {
	out, err := sshExec(fmt.Sprintf(
		"sudo kubectl get pods -n %s --no-headers 2>&1", globalPodNS))
	if err != nil {
		return msgBGPods{err: err}
	}
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return msgBGPods{lines: lines}
}

// battlegroupUpdate handles all messages for the Battlegroup tab.
func battlegroupUpdate(msg tea.Msg, m model) (model, tea.Cmd) {
	bg := &m.bg

	switch msg := msg.(type) {
	case msgBGStatus:
		bg.loading = false
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
		} else {
			bg.result = msg.output
			m.statusMsg, m.statusIsOK = "Status refreshed", true
		}
		return m, nil

	case msgBGExec:
		bg.loading = false
		bg.confirm = false
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
		} else {
			bg.result = msg.output
			m.statusMsg, m.statusIsOK = "Done", true
		}
		return m, nil

	case msgBGPods:
		bg.loading = false
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
		} else {
			bg.pods = msg.lines
			bg.selectedPod = 0
			bg.shellCmd = ""
		}
		return m, nil

	case tea.KeyPressMsg:
		k := msg.String()

		// y/n confirmation prompt
		if bg.confirm {
			switch k {
			case "y", "Y":
				bg.loading = true
				return m, bgExecForSubView(bg.subView)
			case "n", "N", "esc":
				bg.confirm = false
			}
			return m, nil
		}

		// pod navigation when pods are loaded
		if bg.subView == bgvPods && len(bg.pods) > 0 {
			switch k {
			case "up", "k":
				if bg.selectedPod > 0 {
					bg.selectedPod--
					bg.shellCmd = ""
				}
			case "down", "j":
				if bg.selectedPod < len(bg.pods)-1 {
					bg.selectedPod++
					bg.shellCmd = ""
				}
			case "enter":
				fields := strings.Fields(bg.pods[bg.selectedPod])
				if len(fields) > 0 {
					pod := fields[0]
					bg.shellCmd = fmt.Sprintf(
						"ssh -i %s %s@%s sudo kubectl exec -it -n %s %s -- /bin/sh",
						resolveKeyPath(), sshUser, sshHost, globalPodNS, pod)
				}
			case "esc":
				bg.subView = bgvMenu
				bg.pods = nil
				bg.shellCmd = ""
			}
			return m, nil
		}

		// main menu navigation
		switch k {
		case "up", "k":
			if bg.menu > 0 {
				bg.menu--
			}
		case "down", "j":
			if bg.menu < len(bgMenuLabels)-1 {
				bg.menu++
			}
		case "r":
			bg.loading = true
			bg.subView = bgvStatus
			return m, tea.Cmd(cmdBGStatus)
		case "esc":
			bg.confirm = false
			bg.subView = bgvMenu
			bg.result = ""
		case "enter":
			bg.subView = bgSubView(bg.menu + 1) // +1 because bgvMenu=0, bgvStatus=1...
			switch bg.subView {
			case bgvStatus:
				bg.loading = true
				return m, tea.Cmd(cmdBGStatus)
			case bgvPods:
				bg.loading = true
				bg.pods = nil
				return m, tea.Cmd(cmdBGPods)
			case bgvStart:
				bg.loading = true
				return m, cmdBGExec("start")
			case bgvStop, bgvRestart, bgvUpdate, bgvBackup, bgvRestore:
				bg.confirm = true
			}
		}
	}
	return m, nil
}

func bgExecForSubView(sv bgSubView) tea.Cmd {
	switch sv {
	case bgvStart:
		return cmdBGExec("start")
	case bgvStop:
		return cmdBGExec("stop")
	case bgvRestart:
		return cmdBGExec("restart")
	case bgvUpdate:
		return cmdBGExec("update")
	case bgvBackup:
		return cmdBGExec("backup")
	case bgvRestore:
		return cmdBGExec("restore")
	}
	return nil
}

// battlegroupView renders the Battlegroup tab.
func battlegroupView(m model) string {
	bg := m.bg
	menuW := 22
	contentW := m.width - menuW - 5
	if contentW < 10 {
		contentW = 10
	}

	// Left menu pane
	var menuLines []string
	for i, label := range bgMenuLabels {
		if i == bg.menu {
			menuLines = append(menuLines, styleSelected.Render("▶ "+label))
		} else {
			menuLines = append(menuLines, styleDim.Render("  "+label))
		}
	}
	menuPane := stylePanelBorder.Width(menuW).Render(strings.Join(menuLines, "\n"))

	// Right content pane
	var body string
	switch {
	case bg.loading:
		body = styleDim.Render("  loading…")
	case bg.confirm:
		label := bgMenuLabels[bg.menu]
		body = styleErr.Render("  ⚠  "+label+" — are you sure?") +
			"\n\n  " + styleHelp.Render("[y] yes   [n] cancel")
	case bg.subView == bgvPods:
		if len(bg.pods) == 0 {
			body = styleDim.Render("  No pods found.")
		} else {
			var lines []string
			for i, p := range bg.pods {
				if i == bg.selectedPod {
					lines = append(lines, styleSelected.Render("▶ "+p))
				} else {
					lines = append(lines, "  "+p)
				}
			}
			if bg.shellCmd != "" {
				lines = append(lines, "", styleOK.Render("  Shell command:"))
				lines = append(lines, styleHelp.Render("  "+bg.shellCmd))
			} else {
				lines = append(lines, "", styleDim.Render("  Press Enter to get shell command for selected pod."))
			}
			body = strings.Join(lines, "\n")
		}
	case bg.result != "":
		body = bg.result
	default:
		body = styleDim.Render("  Select an action and press Enter.")
	}

	contentPane := stylePanelBorderFocused.Width(contentW).Render(body)

	help := styleHelp.Render("  ↑↓ navigate   Enter select   r refresh status   Esc back")
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, menuPane, contentPane),
		help,
	)
}
