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
type msgLogPodFound struct {
	pod    string
	source int
	err    error
}

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

// cmdFindLogPod discovers the appropriate pod name asynchronously.
func cmdFindLogPod(source int) tea.Cmd {
	return func() tea.Msg {
		var podGrep string
		switch source {
		case 0:
			podGrep = "server-"
		case 1:
			podGrep = "operator"
		}
		out, err := sshExec(fmt.Sprintf(
			"sudo kubectl get pods -n %s --no-headers 2>/dev/null | grep '%s' | grep -v db | head -1 | awk '{print $1}'",
			globalPodNS, podGrep))
		if err != nil {
			return msgLogPodFound{err: err}
		}
		pod := strings.TrimSpace(out)
		if pod == "" {
			return msgLogPodFound{err: fmt.Errorf("pod not found: %s", podGrep)}
		}
		return msgLogPodFound{pod: pod, source: source}
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
		if !lg.streaming {
			// Orphaned listener from a previous stream — ignore
			return m, nil
		}
		lg.streaming = false
		m.statusMsg, m.statusIsOK = "Stream ended", true
		return m, nil

	case msgLogPodFound:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			lg.subView = lgvMenu
			return m, nil
		}
		cmd := fmt.Sprintf("sudo kubectl logs -f -n %s %s 2>&1", globalPodNS, msg.pod)
		ch, cancel, err := sshStream(cmd)
		if err != nil {
			m.statusMsg, m.statusIsOK = err.Error(), false
			lg.subView = lgvMenu
			return m, nil
		}
		lg.buffer = nil
		lg.streamCh = ch
		lg.cancelFn = cancel
		lg.streaming = true
		lg.autoScroll = true
		lg.vp.SetContent("")
		m.statusMsg = "Streaming: " + msg.pod
		m.statusIsOK = true
		return m, listenForLogLine(ch)

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
				lg.streaming = false
				lg.buffer = nil
				lg.vp.SetContent("")
				return m, cmdFindLogPod(lg.menu)
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
			lg.streaming = false
			lg.subView = lgvStream
			return m, cmdFindLogPod(lg.menu)
		}
	}
	return m, nil
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
