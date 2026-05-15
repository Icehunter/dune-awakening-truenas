package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── config ────────────────────────────────────────────────────────────────────

var (
	sshHost      string
	sshUser      string
	sshKeyPath   string
	itemDataPath string
	scripCurrencyID int
	dbPort       int
	dbUser       string
	dbPass       string
	dbName       string
	dbSchema     string
)

func init() {
	flag.StringVar(&sshHost, "host", "192.168.0.72:22", "SSH host:port")
	flag.StringVar(&sshUser, "user", "dune", "SSH user")
	flag.StringVar(&sshKeyPath, "key", "", "SSH private key path (auto-detected if empty)")
	flag.StringVar(&itemDataPath, "itemdata", "", "Item data JSON path (stack_max/volume overrides)")
	flag.IntVar(&scripCurrencyID, "scripcurrency", -1, "Scrip currency id (auto-detect if -1)")
	flag.IntVar(&dbPort, "dbport", 15432, "PostgreSQL port inside the cluster")
	flag.StringVar(&dbUser, "dbuser", "dune", "PostgreSQL user")
	flag.StringVar(&dbPass, "dbpass", "dune", "PostgreSQL password")
	flag.StringVar(&dbName, "dbname", "dune", "PostgreSQL database name")
	flag.StringVar(&dbSchema, "schema", "dune", "PostgreSQL schema")
}

func resolveKeyPath() string {
	if sshKeyPath != "" {
		return sshKeyPath
	}
	candidates := []string{
		"../sshKey",
		"./sshKey",
		filepath.Join(os.Getenv("HOME"), ".ssh", "dune"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0]
}

func resolveItemDataPath() string {
	if itemDataPath != "" {
		return itemDataPath
	}
	candidates := []string{
		"./item-data.json",
		"../item-data.json",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func loadItemData() error {
	path := resolveItemDataPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read item data %s: %w", path, err)
	}
	var parsed itemDataFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parse item data %s: %w", path, err)
	}
	if parsed.Items == nil {
		parsed.Items = map[string]itemRule{}
	}
	itemData = parsed
	return nil
}

// ── domain types ─────────────────────────────────────────────────────────────

type playerInfo struct {
	ID        int64
	AccountID int64
	Class     string
	Map       string
	FactionID int16
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
	StackMax int64   `json:"stack_max"`
	Volume   float64 `json:"volume"`
}

type itemDataFile struct {
	DefaultStackMax int64               `json:"default_stack_max"`
	DefaultVolume   float64             `json:"default_volume"`
	Items           map[string]itemRule `json:"items"`
}

var itemData itemDataFile

// ── multi-step input ──────────────────────────────────────────────────────────

type inputStep struct {
	prompt string
	hint   string
	value  string
}

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

// ── model ─────────────────────────────────────────────────────────────────────

type model struct {
	width  int
	height int

	activeTab  int
	connected  bool
	statusMsg  string
	statusIsOK bool

	pl PlayersState
	// bg, db, lg will be added in Tasks 5, 7, 8
}

// ── init ──────────────────────────────────────────────────────────────────────

func initialModel() model {
	return model{
		pl: newPlayersState(),
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
		return m, nil

	case msgConnect:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
		} else {
			m.connected = true
			m.statusMsg = "Connected → " + sshHost
			m.statusIsOK = true
			m.activeTab = 1
		}
		return m, nil

	case tea.KeyPressMsg:
		k := msg.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		}
		switch k {
		case "1":
			m.activeTab = 0
			return m, nil
		case "2":
			m.activeTab = 1
			return m, nil
		case "3":
			m.activeTab = 2
			return m, nil
		case "4":
			m.activeTab = 3
			return m, nil
		}
	}

	// delegate to active tab
	switch m.activeTab {
	case 1:
		return playersUpdate(msg, m)
	}
	return m, nil
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
	connDot := styleDim.Render("○ connecting…")
	if m.connected {
		connDot = styleOK.Render("● " + sshHost)
	}
	titleText := styleTitle.Render("DUNE AWAKENING ADMIN")
	leftW := outerW - lipgloss.Width(connDot)
	if leftW < 1 {
		leftW = 1
	}
	titleBar := lipgloss.PlaceHorizontal(leftW, lipgloss.Left, titleText) + connDot

	// ── status/help bar ───────────────────────────────────────────────────────
	var bottomLine string
	if m.statusMsg != "" {
		if m.statusIsOK {
			bottomLine = styleOK.Render("✓ " + m.statusMsg)
		} else {
			bottomLine = styleErr.Render("✗ " + m.statusMsg)
		}
	} else {
		bottomLine = styleHelp.Render("↑↓/jk move  enter select  esc back  tab autocomplete  q quit")
	}

	// ── body ──────────────────────────────────────────────────────────────────
	var body string
	if !m.connected {
		body = m.renderConnectingPane()
	} else {
		switch m.activeTab {
		case 1:
			body = playersView(m)
		default:
			body = m.renderPlaceholder()
		}
	}

	// ── assemble ──────────────────────────────────────────────────────────────
	var sb strings.Builder
	sb.WriteString(titleBar)
	sb.WriteString("\n")
	sb.WriteString(body)
	sb.WriteString("\n")
	sb.WriteString(bottomLine)
	return sb.String()
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

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()
	if err := loadItemData(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if globalDB != nil {
		globalDB.Close(context.Background())
	}
	if globalSSH != nil {
		globalSSH.Close()
	}
}
