package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type logSubView int

const (
	lgvMenu logSubView = iota
	lgvStream
)

// LogsState holds all state for the Logs tab.
type LogsState struct {
	menu       int
	subView    logSubView
	vp         viewport.Model
	buffer     []string
	autoScroll bool
	streaming  bool
	streamCh   <-chan string
	cancelFn   func()
}

var logMenuLabels = []string{"Server Logs", "Operator Logs"}

// Message types
type msgLogLine struct{ line string }
type msgLogDone struct{}

func newLogsState() LogsState {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(24))
	return LogsState{vp: vp, autoScroll: true}
}

// listenForLogLine blocks until the next line on ch.
func listenForLogLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return msgLogDone{}
		}
		return msgLogLine{line: line}
	}
}

// logsUpdate handles all messages for the Logs tab.
func logsUpdate(msg tea.Msg, m model) (model, tea.Cmd) {
	lg := &m.lg

	switch msg := msg.(type) {
	case msgLogLine:
		lg.buffer = append(lg.buffer, msg.line)
		lg.vp.SetContent(strings.Join(lg.buffer, "\n"))
		if lg.autoScroll {
			lg.vp.GotoBottom()
		}
		return m, listenForLogLine(lg.streamCh)

	case msgLogDone:
		lg.streaming = false
		m.statusMsg, m.statusIsOK = "Stream ended", true
		return m, nil

	case tea.WindowSizeMsg:
		lg.vp.SetWidth(msg.Width - 4)
		h := msg.Height - 8
		if h < 5 {
			h = 5
		}
		lg.vp.SetHeight(h)
		return m, nil

	case tea.KeyPressMsg:
		k := msg.String()

		if lg.subView == lgvStream {
			switch k {
			case "s":
				if lg.cancelFn != nil {
					lg.cancelFn()
					lg.cancelFn = nil
				}
				lg.streaming = false
				m.statusMsg, m.statusIsOK = "Stream stopped", true
				return m, nil
			case "r":
				// Restart: cancel current stream, start fresh
				if lg.cancelFn != nil {
					lg.cancelFn()
					lg.cancelFn = nil
				}
				lg.buffer = nil
				lg.vp.SetContent("")
				return startLogStream(m, lg.menu)
			case "e":
				fname := fmt.Sprintf("%s/dune-logs-%d.txt", os.Getenv("HOME"), time.Now().Unix())
				_ = os.WriteFile(fname, []byte(strings.Join(lg.buffer, "\n")), 0644)
				m.statusMsg = "Saved: " + fname
				m.statusIsOK = true
				return m, nil
			case "esc":
				if lg.cancelFn != nil {
					lg.cancelFn()
					lg.cancelFn = nil
				}
				lg.streaming = false
				lg.subView = lgvMenu
				return m, nil
			case "end":
				lg.autoScroll = true
				lg.vp.GotoBottom()
				return m, nil
			default:
				// Any scroll key pauses auto-scroll
				lg.autoScroll = false
				var cmd tea.Cmd
				lg.vp, cmd = lg.vp.Update(msg)
				return m, cmd
			}
		}

		// lgvMenu
		switch k {
		case "up", "k":
			if lg.menu > 0 {
				lg.menu--
			}
		case "down", "j":
			if lg.menu < len(logMenuLabels)-1 {
				lg.menu++
			}
		case "enter":
			return startLogStream(m, lg.menu)
		}
	}
	return m, nil
}

// startLogStream discovers the appropriate pod and begins streaming its logs.
func startLogStream(m model, source int) (model, tea.Cmd) {
	lg := &m.lg

	var podGrep string
	switch source {
	case 0:
		podGrep = "server-"
	case 1:
		podGrep = "operator"
	}

	podOut, err := sshExec(fmt.Sprintf(
		"sudo kubectl get pods -n %s --no-headers 2>/dev/null | grep '%s' | grep -v db | head -1 | awk '{print $1}'",
		globalPodNS, podGrep))
	if err != nil || strings.TrimSpace(podOut) == "" {
		m.statusMsg = "Pod not found: " + podGrep
		m.statusIsOK = false
		return m, nil
	}

	pod := strings.TrimSpace(podOut)
	cmd := fmt.Sprintf("sudo kubectl logs -f -n %s %s 2>&1", globalPodNS, pod)
	ch, cancel, err := sshStream(cmd)
	if err != nil {
		m.statusMsg = err.Error()
		m.statusIsOK = false
		return m, nil
	}

	lg.buffer = nil
	lg.streamCh = ch
	lg.cancelFn = cancel
	lg.streaming = true
	lg.autoScroll = true
	lg.subView = lgvStream
	lg.vp.SetContent("")
	m.statusMsg = "Streaming: " + pod
	m.statusIsOK = true
	return m, listenForLogLine(ch)
}

// logsView renders the Logs tab.
func logsView(m model) string {
	lg := m.lg

	if lg.subView == lgvStream {
		var header string
		if lg.streaming {
			header = styleOK.Render("  ● "+logMenuLabels[lg.menu]) +
				styleDim.Render("  s=stop  e=export  End=bottom  Esc=menu")
		} else {
			header = styleDim.Render("  ○ "+logMenuLabels[lg.menu]+" (stopped)") +
				styleHelp.Render("  r=restart  e=export  Esc=menu")
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			stylePanelBorder.Width(m.width-2).Render(lg.vp.View()),
		)
	}

	// Menu view
	menuW := 22
	contentW := m.width - menuW - 5
	if contentW < 10 {
		contentW = 10
	}

	var menuLines []string
	for i, label := range logMenuLabels {
		if i == lg.menu {
			menuLines = append(menuLines, styleSelected.Render("▶ "+label))
		} else {
			menuLines = append(menuLines, styleDim.Render("  "+label))
		}
	}
	menuPane := stylePanelBorder.Width(menuW).Render(strings.Join(menuLines, "\n"))
	contentPane := stylePanelBorderFocused.Width(contentW).Render(
		styleDim.Render("  Select a log source and press Enter to start streaming.\n\n") +
			styleHelp.Render("  s=stop   r=restart   e=export   End=scroll to bottom   Esc=menu"),
	)
	return lipgloss.JoinHorizontal(lipgloss.Top, menuPane, contentPane)
}
