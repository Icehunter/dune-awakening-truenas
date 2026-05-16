package main

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// dbItemTemplates is the merged, sorted list of item templates from the DB +
// item-data.json + dune-item-names.json. Populated after connect.
var dbItemTemplates []string

// duneItemNames is keyed by lowercase template ID; holds PascalCase ID and
// English display name from dune-item-names.json.
type duneItemName struct {
	ID   string
	Name string
}

var duneItemNames map[string]duneItemName

// ── tab constants ─────────────────────────────────────────────────────────────

const (
	tabBattlegroup = 0
	tabPlayers     = 1
	tabDatabase    = 2
	tabLogs        = 3
)

var tabLabels = [4]string{"Battlegroup", "Players", "Database", "Logs"}

// ── palette / styles ─────────────────────────────────────────────────────────

var (
	clrOrange = lipgloss.Color("#FF6600")
	clrDim    = lipgloss.Color("#555555")
	clrWhite  = lipgloss.Color("#DDDDDD")
	clrGreen  = lipgloss.Color("#44FF88")
	clrRed    = lipgloss.Color("#FF4444")
	clrBlue   = lipgloss.Color("#44AAFF")
	clrBg     = lipgloss.Color("#0D0D0D")
	clrPanel  = lipgloss.Color("#1A1A1A")

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrOrange)

	styleDim = lipgloss.NewStyle().
			Foreground(clrDim)

	styleHelp = lipgloss.NewStyle().
			Foreground(clrDim)

	styleErr = lipgloss.NewStyle().
			Foreground(clrRed).Bold(true)

	styleOK = lipgloss.NewStyle().
		Foreground(clrGreen).Bold(true)

	styleSelected = lipgloss.NewStyle().
			Foreground(clrOrange).Bold(true)

	styleNormal = lipgloss.NewStyle().
			Foreground(clrWhite)

	stylePanelBorder = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(clrDim)

	stylePanelBorderFocused = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(clrOrange)

	styleTableSel = lipgloss.NewStyle().
			Foreground(clrBg).
			Background(clrOrange).
			Bold(true)

	styleTableHdr = lipgloss.NewStyle().
			Foreground(clrOrange).
			Bold(true)
)

// ── domain types ─────────────────────────────────────────────────────────────

type playerInfo struct {
	ID           int64 // pawn actor (PlayerCharacter) — used for inventory, give-item, award-XP
	AccountID    int64
	ControllerID int64 // PlayerController actor — used for currency, faction rep, scrip
	Name         string
	Class        string
	Map          string
	FactionID    int16
}

type itemInfo struct {
	ID         int64
	TemplateID string
	StackSize  int64
	Quality    int64
	Durability string
}

type currencyRow struct {
	PlayerID   int64
	CurrencyID int16
	Balance    int64
}

type factionRep struct {
	ActorID     int64
	FactionID   int16
	FactionName string
	Reputation  int32
	Scrips      int64
}

type specTrack struct {
	PlayerID  int64
	TrackType string
	XP        int32
	Level     float32
}

type itemRule struct {
	Name     string  `json:"name"`
	StackMax int64   `json:"stack_max"`
	Volume   float64 `json:"volume"`
	Tier     int     `json:"tier"`
	Rarity   string  `json:"rarity"`
}

type itemDataFile struct {
	DefaultStackMax int64               `json:"default_stack_max"`
	DefaultVolume   float64             `json:"default_volume"`
	Items           map[string]itemRule `json:"items"`
}

// ── multi-step input ──────────────────────────────────────────────────────────

type inputStep struct {
	prompt string
	hint   string
	value  string
}

// ── messages ─────────────────────────────────────────────────────────────────

type msgConnect struct{ err error }
type msgPlayers struct {
	rows []playerInfo
	err  error
}
type msgInventory struct {
	rows []itemInfo
	err  error
}
type msgCurrency struct {
	rows []currencyRow
	err  error
}
type msgFactions struct {
	rows            []factionRep
	scripCurrencyID int16
	err             error
}
type msgSpecs struct {
	rows []specTrack
	err  error
}
type msgSQL struct {
	result string
	err    error
}
type msgMutate struct {
	ok  string
	err error
}
type msgItemTemplates struct {
	templates []string
}

// ── model ─────────────────────────────────────────────────────────────────────

type model struct {
	width  int
	height int

	activeTab  int
	connected  bool
	statusMsg  string
	statusIsOK bool

	pl PlayersState
	bg BattlegroupState
	db DatabaseState
	lg LogsState
}

// ── init ──────────────────────────────────────────────────────────────────────

func initialModel() model {
	return model{
		pl: newPlayersState(),
		db: newDatabaseState(),
		lg: newLogsState(),
	}
}

func (m model) Init() tea.Cmd {
	return cmdConnect
}

// ── update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = rebuildPlayersTable(m)
		// Resize the logs viewport to match the new terminal size
		m.lg.vp.SetWidth(msg.Width - 4)
		h := msg.Height - 8
		if h < 5 {
			h = 5
		}
		m.lg.vp.SetHeight(h)
		return m, nil

	case msgConnect:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
		} else {
			m.connected = true
			m.statusMsg = "Connected → " + sshHost
			m.statusIsOK = true
			m.activeTab = tabPlayers
			return m, tea.Batch(tea.Cmd(cmdFetchPlayers), tea.Cmd(cmdFetchItemTemplates))
		}
		return m, nil

	case msgItemTemplates:
		// Merge (in priority order): DB templates → duneItemNames → itemData keys.
		// DB and duneItemNames supply correct PascalCase; itemData fills gaps.
		seen := make(map[string]string) // lowercase → preferred-case template ID
		for _, t := range msg.templates {
			seen[strings.ToLower(t)] = t
		}
		for k, v := range duneItemNames {
			if _, ok := seen[k]; !ok {
				seen[k] = v.ID
			}
		}
		if itemData.Items != nil {
			for k := range itemData.Items {
				if _, ok := seen[k]; !ok {
					seen[k] = k
				}
			}
		}
		merged := make([]string, 0, len(seen))
		for _, v := range seen {
			merged = append(merged, v)
		}
		sort.Strings(merged)
		dbItemTemplates = merged
		return m, nil

	case tea.KeyPressMsg:
		k := msg.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		}
		// Only switch tabs when not inside a text input wizard
		if !playersIsInputState(m) {
			switch k {
			case "1":
				m.activeTab = tabBattlegroup
				return m, nil
			case "2":
				m.activeTab = tabPlayers
				return m, nil
			case "3":
				m.activeTab = tabDatabase
				return m, nil
			case "4":
				m.activeTab = tabLogs
				if len(m.lg.pods) == 0 && !m.lg.loadingPods {
					m.lg.loadingPods = true
					return m, tea.Cmd(cmdFetchLogPods)
				}
				return m, nil
			}
		}
	}

	// Delegate to the active tab's update handler.
	switch m.activeTab {
	case tabBattlegroup:
		return battlegroupUpdate(msg, m)
	case tabDatabase:
		return databaseUpdate(msg, m)
	case tabLogs:
		return logsUpdate(msg, m)
	}
	// Players is the default — playersUpdate handles its own message types
	// and returns m, nil for anything it doesn't recognise.
	return playersUpdate(msg, m)
}

// ── table helpers ─────────────────────────────────────────────────────────────

// tableArea returns the width/height available for a table inside the right
// content pane (accounting for panel border).
func (m model) tableArea() (w, h int) {
	menuW := 24
	bodyH := m.height - 2 // title + bottom bar

	contentW := m.width - menuW - 1 // gap between panes
	innerW := contentW - 2          // panel left+right border
	innerH := bodyH - 2             // panel top+bottom border

	if innerW < 10 {
		innerW = 10
	}
	if innerH < 4 {
		innerH = 4
	}
	return innerW, innerH
}

func newTable(cols []table.Column, rows []table.Row, w, h int, s table.Styles) table.Model {
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithStyles(s),
	)
	t.SetWidth(w)
	t.SetHeight(h)
	return t
}

// ── view ──────────────────────────────────────────────────────────────────────

func (m model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("Loading…")
		v.AltScreen = true
		return v
	}

	content := m.renderFrame()
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m model) renderFrame() string {
	outerW := m.width

	// ── title bar ─────────────────────────────────────────────────────────────
	tabBar := renderTabBar(m)

	// ── status/help bar ───────────────────────────────────────────────────────
	statusBar := renderStatusBar(m)

	// ── body ──────────────────────────────────────────────────────────────────
	var body string
	if !m.connected {
		body = m.renderConnectingPane()
	} else {
		switch m.activeTab {
		case tabBattlegroup:
			body = battlegroupView(m)
		case tabPlayers:
			body = playersView(m)
		case tabDatabase:
			body = databaseView(m)
		case tabLogs:
			body = logsView(m)
		default:
			body = m.renderPlaceholder()
		}
	}

	// ── assemble ──────────────────────────────────────────────────────────────
	_ = outerW // used implicitly via m.width in helpers
	var sb strings.Builder
	sb.WriteString(tabBar)
	sb.WriteString("\n")
	sb.WriteString(body)
	sb.WriteString("\n")
	sb.WriteString(statusBar)
	return sb.String()
}

func renderTabBar(m model) string {
	var parts []string
	for i, label := range tabLabels {
		s := fmt.Sprintf(" %d:%s ", i+1, label)
		if i == m.activeTab {
			parts = append(parts, styleSelected.Render(s))
		} else {
			parts = append(parts, styleDim.Render(s))
		}
	}
	bar := strings.Join(parts, styleDim.Render("│"))
	return lipgloss.NewStyle().
		Width(m.width).
		Background(clrPanel).
		Render(bar)
}

func renderStatusBar(m model) string {
	dot := styleDim.Render("○")
	if m.connected {
		dot = styleOK.Render("●")
	}
	msg := m.statusMsg
	if msg != "" {
		if m.statusIsOK {
			msg = styleOK.Render(msg)
		} else {
			msg = styleErr.Render(msg)
		}
	} else {
		msg = styleHelp.Render("↑↓/jk move  enter select  esc back  tab autocomplete  q quit")
	}
	return lipgloss.NewStyle().
		Width(m.width).
		Background(clrPanel).
		Render(fmt.Sprintf(" %s %s  %s", dot, sshHost, msg))
}

func (m model) renderConnectingPane() string {
	bodyH := m.height - 2
	if bodyH < 4 {
		bodyH = 4
	}
	menuW := 24
	contentW := m.width - menuW - 1

	inner := bodyH - 2
	body := styleDim.Render("\n  Establishing SSH tunnel to " + sshHost + "…")

	rendered := stylePanelBorder.Width(contentW).Height(inner).Render(body)
	content := overlayTitle(rendered, " Connecting… ")

	// empty menu pane for layout
	menuRendered := stylePanelBorder.Width(menuW).Height(inner).Render("")
	menuPane := overlayTitle(menuRendered, " Menu ")

	return lipgloss.JoinHorizontal(lipgloss.Top, menuPane, content)
}

func (m model) renderPlaceholder() string {
	bodyH := m.height - 2
	if bodyH < 4 {
		bodyH = 4
	}
	menuW := 24
	contentW := m.width - menuW - 1
	inner := bodyH - 2

	body := styleDim.Render("\n  (tab not yet implemented)")
	rendered := stylePanelBorder.Width(contentW).Height(inner).Render(body)
	content := overlayTitle(rendered, " — ")

	menuRendered := stylePanelBorder.Width(menuW).Height(inner).Render("")
	menuPane := overlayTitle(menuRendered, " Menu ")

	return lipgloss.JoinHorizontal(lipgloss.Top, menuPane, content)
}

// overlayTitle writes a title string over the top border of a rendered box.
func overlayTitle(box, title string) string {
	if title == "" {
		return box
	}
	lines := strings.Split(box, "\n")
	if len(lines) == 0 {
		return box
	}

	raw := lines[0]
	ansiPrefix, plainTop, ansiSuffix := splitANSI(raw)

	top := []rune(plainTop)
	t := []rune(" " + strings.TrimSpace(title) + " ")
	if len(t)+2 <= len(top) {
		copy(top[2:], t)
		lines[0] = ansiPrefix + string(top) + ansiSuffix
	}
	return strings.Join(lines, "\n")
}

// splitANSI splits a string of the form "<ANSI>text<reset>" into its three parts.
func splitANSI(s string) (prefix, middle, suffix string) {
	i := 0
	for i < len(s) {
		if s[i] != '\x1b' {
			break
		}
		j := i + 1
		for j < len(s) && s[j] != 'm' {
			j++
		}
		if j < len(s) {
			j++
		}
		i = j
	}
	prefix = s[:i]
	rest := s[i:]

	if idx := strings.LastIndex(rest, "\x1b["); idx >= 0 {
		middle = rest[:idx]
		suffix = rest[idx:]
	} else {
		middle = rest
		suffix = ""
	}
	return
}
