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
	lgvPodList logSubView = iota
	lgvStream
)

// logPod identifies a pod by namespace and name.
type logPod struct {
	namespace string
	name      string
}

// LogsState holds all state for the Logs tab.
type LogsState struct {
	subView logSubView
	// pod list (menu)
	pods        []logPod
	podCursor   int
	loadingPods bool
	// streaming
	vp         viewport.Model
	buffer     []string
	autoScroll bool
	streaming  bool
	streamCh   <-chan string
	cancelFn   func()
	currentPod string // name of pod being tailed
}

// Message types
type msgLogLine struct{ line string }
type msgLogDone struct{}
type msgLogPods struct {
	pods []logPod
	err  error
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

// cmdFetchLogPods discovers pods in the battlegroup and operators namespaces.
func cmdFetchLogPods() tea.Msg {
	var pods []logPod

	// Fetch from battlegroup namespace
	out, err := sshExec(fmt.Sprintf(
		"sudo kubectl get pods -n %s --no-headers -o custom-columns=NAME:.metadata.name 2>/dev/null",
		globalPodNS))
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			name := strings.TrimSpace(line)
			if name != "" && !strings.Contains(name, "db-dbdepl") {
				pods = append(pods, logPod{namespace: globalPodNS, name: name})
			}
		}
	}

	// Fetch from funcom-operators namespace
	out2, err2 := sshExec("sudo kubectl get pods -n funcom-operators --no-headers -o custom-columns=NAME:.metadata.name 2>/dev/null")
	if err2 == nil {
		for _, line := range strings.Split(strings.TrimSpace(out2), "\n") {
			name := strings.TrimSpace(line)
			if name != "" {
				pods = append(pods, logPod{namespace: "funcom-operators", name: name})
			}
		}
	}

	if len(pods) == 0 {
		return msgLogPods{err: fmt.Errorf("no pods found")}
	}
	return msgLogPods{pods: pods}
}

// startLogStreamFromPod starts tailing logs for the given pod.
func startLogStreamFromPod(m model, pod logPod) (model, tea.Cmd) {
	lg := &m.lg
	cmd := fmt.Sprintf("sudo kubectl logs -f -n %s %s 2>&1", pod.namespace, pod.name)
	ch, cancel, err := sshStream(cmd)
	if err != nil {
		m.statusMsg, m.statusIsOK = err.Error(), false
		return m, nil
	}
	lg.buffer = nil
	lg.streamCh = ch
	lg.cancelFn = cancel
	lg.streaming = true
	lg.autoScroll = true
	lg.subView = lgvStream
	lg.currentPod = pod.name
	lg.vp.SetContent("")
	m.statusMsg = "Streaming: " + pod.name
	m.statusIsOK = true
	return m, listenForLogLine(ch)
}

// logsUpdate handles all messages for the Logs tab.
func logsUpdate(msg tea.Msg, m model) (model, tea.Cmd) {
	lg := &m.lg

	switch msg := msg.(type) {
	case msgLogLine:
		if !lg.streaming {
			// Orphaned line from a cancelled stream — discard
			return m, nil
		}
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

	case msgLogPods:
		lg.loadingPods = false
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
		} else {
			lg.pods = msg.pods
			lg.podCursor = 0
		}
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
				// Restart: cancel current stream, go back to pod list
				if lg.cancelFn != nil {
					lg.cancelFn()
					lg.cancelFn = nil
				}
				lg.streaming = false
				lg.buffer = nil
				lg.vp.SetContent("")
				// Re-stream the same pod
				if lg.currentPod != "" {
					for _, p := range lg.pods {
						if p.name == lg.currentPod {
							return startLogStreamFromPod(m, p)
						}
					}
				}
				return m, nil
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
				lg.subView = lgvPodList
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

		// lgvPodList
		switch k {
		case "up", "k":
			if lg.podCursor > 0 {
				lg.podCursor--
			}
		case "down", "j":
			if lg.podCursor < len(lg.pods)-1 {
				lg.podCursor++
			}
		case "r":
			lg.loadingPods = true
			lg.pods = nil
			return m, tea.Cmd(cmdFetchLogPods)
		case "enter":
			if len(lg.pods) > 0 {
				return startLogStreamFromPod(m, lg.pods[lg.podCursor])
			}
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
			header = styleOK.Render("  ● "+lg.currentPod) +
				styleDim.Render("  s=stop  e=export  End=bottom  Esc=pod list")
		} else {
			header = styleDim.Render("  ○ "+lg.currentPod+" (stopped)") +
				styleHelp.Render("  r=restart  e=export  Esc=pod list")
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			stylePanelBorder.Width(m.width-2).Render(lg.vp.View()),
		)
	}

	// Pod list view
	inner := m.height - 4
	if inner < 5 {
		inner = 5
	}
	menuW := 24
	contentW := m.width - menuW - 1
	if contentW < 10 {
		contentW = 10
	}

	// Left: info pane — pre-pad to inner lines
	infoText := styleDim.Render("  Select a pod\n  to tail its logs.\n\n") +
		styleHelp.Render("  Enter=stream\n  r=refresh\n  Esc=back")
	infoSplit := strings.Split(infoText, "\n")
	for len(infoSplit) < inner {
		infoSplit = append(infoSplit, "")
	}
	menuPane := stylePanelBorder.Width(menuW).Height(inner).Render(strings.Join(infoSplit, "\n"))

	// Right: scrollable pod list
	var body string
	if lg.loadingPods {
		body = styleDim.Render("  loading pods…")
	} else if len(lg.pods) == 0 {
		body = styleDim.Render("  No pods found.\n  Press r to refresh.")
	} else {
		bgShortName := strings.TrimPrefix(globalPodNS, "funcom-seabass-")
		var lines []string
		for i, p := range lg.pods {
			label := p.name
			if p.namespace == globalPodNS {
				label = strings.TrimPrefix(p.name, bgShortName+"-")
			}
			ns := ""
			if p.namespace == "funcom-operators" {
				ns = styleHelp.Render(" [op]")
			}
			if i == lg.podCursor {
				lines = append(lines, styleSelected.Render("▶ "+label)+ns)
			} else {
				lines = append(lines, styleDim.Render("  "+label)+ns)
			}
		}
		body = strings.Join(lines, "\n")
	}

	bodyLines := strings.Split(body, "\n")
	for len(bodyLines) < inner {
		bodyLines = append(bodyLines, "")
	}
	contentPane := stylePanelBorderFocused.Width(contentW).Height(inner).Render(strings.Join(bodyLines, "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, menuPane, contentPane)
}
