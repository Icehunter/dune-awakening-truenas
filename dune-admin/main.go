package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── config ────────────────────────────────────────────────────────────────────

var (
	sshHost         string
	sshUser         string
	sshKeyPath      string
	itemDataPath    string
	scripCurrencyID int
	dbPort          int
	dbUser          string
	dbPass          string
	dbName          string
	dbSchema        string
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

// ── view state ────────────────────────────────────────────────────────────────

type viewState int

const (
	viewConnect viewState = iota
	viewMenu
	viewPlayers
	viewInventory
	viewCurrency
	viewFactions
	viewFactionRep
	viewSpecializations
	viewGiveItem
	viewGiveCurrency
	viewGiveFactionRep
	viewGiveLandsraadScrip
	viewAwardXP
	viewSQL
	viewSQLResult
)

// ── multi-step input ──────────────────────────────────────────────────────────

type inputStep struct {
	prompt string
	hint   string
	value  string
}

// ── menu items ────────────────────────────────────────────────────────────────

type menuItem struct {
	label string
	state viewState
}

var menuItems = []menuItem{
	{"Players", viewPlayers},
	{"Inventory", viewInventory},
	{"Currency", viewCurrency},
	{"Factions & Rep", viewFactions},
	{"Specializations / XP", viewSpecializations},
	{"Give Item", viewGiveItem},
	{"Give Currency", viewGiveCurrency},
	{"Give Faction Rep", viewGiveFactionRep},
	{"Give Landsraad Scrip", viewGiveLandsraadScrip},
	{"Award XP", viewAwardXP},
	{"SQL Query", viewSQL},
	{"Quit", viewMenu},
}

var menuQuitIdx = len(menuItems) - 1

var itemTemplates = []string{
	"Stone", "MelangeSpice", "SpiceResidue", "SteelBar", "Oil", "ScrapMetal",
	"T2MachineComponent", "T3MiningGalleryComponent1", "WormTooth",
	"Ammo", "HeavyAmmo", "HealthPack_Channeled_3", "Solaris",
	"PowerPack", "MiningTool_1h_Standard", "BasicBuildingTool",
	"OrnithopterLightEngine_4", "VehicleBackupTool",
}

// ── model ─────────────────────────────────────────────────────────────────────

type model struct {
	width  int
	height int

	state      viewState
	prevState  viewState
	menuCursor int
	connected  bool

	// fetched data
	players         []playerInfo
	inventory       []itemInfo
	currencies      []currencyRow
	factions        []factionRep
	specs           []specTrack
	sqlResult       string
	scripCurrencyID int16

	// active table widget
	tbl table.Model

	// selected context for drill-down / give commands
	selectedPlayerIdx int
	selectedFactionID int16

	// multi-step wizard
	inputSteps  []inputStep
	inputCursor int
	textInput   textinput.Model

	// status bar
	statusMsg  string
	statusIsOK bool
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

// ── init ──────────────────────────────────────────────────────────────────────

func initialModel() model {
	ti := textinput.New()
	ti.CharLimit = 256
	return model{
		state:     viewConnect,
		textInput: ti,
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
		m.rebuildTable()
		return m, nil

	case msgConnect:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusIsOK = false
		} else {
			m.connected = true
			m.statusMsg = "Connected → " + sshHost
			m.statusIsOK = true
			m.state = viewMenu
		}
		return m, nil

	case msgPlayers:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusIsOK = false
			m.state = viewMenu
		} else {
			m.players = msg.rows
			m.state = viewPlayers
			m.rebuildTable()
		}
		return m, nil

	case msgInventory:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusIsOK = false
			m.state = m.prevState
		} else {
			m.inventory = msg.rows
			m.state = viewInventory
			m.rebuildTable()
		}
		return m, nil

	case msgCurrency:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusIsOK = false
			m.state = viewMenu
		} else {
			m.currencies = msg.rows
			m.state = viewCurrency
			m.rebuildTable()
		}
		return m, nil

	case msgFactions:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusIsOK = false
			m.state = viewMenu
		} else {
			m.factions = msg.rows
			m.scripCurrencyID = msg.scripCurrencyID
			m.state = viewFactions
			m.rebuildTable()
		}
		return m, nil

	case msgSpecs:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusIsOK = false
			m.state = viewMenu
		} else {
			m.specs = msg.rows
			m.state = viewSpecializations
			m.rebuildTable()
		}
		return m, nil

	case msgSQL:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusIsOK = false
			m.state = viewMenu
		} else {
			m.sqlResult = msg.result
			m.state = viewSQLResult
			m.rebuildTable()
		}
		return m, nil

	case msgMutate:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusIsOK = false
		} else {
			m.statusMsg = msg.ok
			m.statusIsOK = true
		}
		m.state = viewMenu
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// global: always allow ctrl+c / q from menu
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if k == "q" && m.state == viewMenu {
		return m, tea.Quit
	}

	// input states — pass full msg so textinput.Update works
	if m.isInputState() {
		return m.handleInputKey(msg)
	}

	// esc navigates back
	if k == "esc" {
		m.statusMsg = ""
		switch m.state {
		case viewInventory:
			m.state = viewPlayers
			m.rebuildTable()
		default:
			m.state = viewMenu
		}
		return m, nil
	}

	switch m.state {
	case viewConnect:
		return m, nil

	case viewMenu:
		return m.handleMenuKey(k)

	case viewPlayers:
		switch k {
		case "enter":
			if len(m.players) > 0 {
				idx := m.tbl.Cursor()
				m.selectedPlayerIdx = idx
				m.prevState = viewPlayers
				return m, cmdFetchInventory(m.players[idx].ID)
			}
		}
		var cmd tea.Cmd
		m.tbl, cmd = m.tbl.Update(msg)
		return m, cmd

	case viewInventory, viewCurrency, viewFactions, viewSpecializations, viewSQLResult:
		var cmd tea.Cmd
		m.tbl, cmd = m.tbl.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) handleMenuKey(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(menuItems)-1 {
			m.menuCursor++
		}
	case "enter":
		return m.activateMenu()
	}
	return m, nil
}

func (m model) activateMenu() (tea.Model, tea.Cmd) {
	m.statusMsg = ""
	idx := m.menuCursor
	if idx == menuQuitIdx {
		return m, tea.Quit
	}
	switch menuItems[idx].state {
	case viewPlayers:
		m.state = viewConnect
		return m, func() tea.Msg { return cmdFetchPlayers() }
	case viewInventory:
		return m.startWizard(viewInventory, []inputStep{
			{prompt: "Player ID", hint: "numeric actor ID (see Players view)"},
		})
	case viewCurrency:
		m.state = viewConnect
		return m, func() tea.Msg { return cmdFetchCurrency() }
	case viewFactions:
		m.state = viewConnect
		return m, func() tea.Msg { return cmdFetchFactions() }
	case viewSpecializations:
		m.state = viewConnect
		return m, func() tea.Msg { return cmdFetchSpecs() }
	case viewGiveItem:
		return m.startWizard(viewGiveItem, []inputStep{
			{prompt: "Player Character ID", hint: "numeric actor ID"},
			{prompt: "Item template", hint: "e.g. MelangeSpice  (Tab to autocomplete)"},
			{prompt: "Quantity", hint: "default: 1"},
			{prompt: "Quality level", hint: "0 = default, 1-4 for higher tier"},
		})
	case viewGiveCurrency:
		return m.startWizard(viewGiveCurrency, []inputStep{
			{prompt: "Player Controller ID", hint: "numeric actor ID"},
			{prompt: "Amount to add", hint: "Solaris delta (use negative to subtract)"},
		})
	case viewGiveFactionRep:
		return m.startWizard(viewGiveFactionRep, []inputStep{
			{prompt: "PlayerController Actor ID", hint: "actor ID where faction rep is stored (see Factions view — use the Actor ID column)"},
			{prompt: "Faction ID", hint: "1=Atreides  2=Harkonnen  4=Smuggler"},
			{prompt: "Scrip delta", hint: "amount to add (negative to subtract) — tier tags auto-synced"},
		})
	case viewGiveLandsraadScrip:
		return m.startWizard(viewGiveLandsraadScrip, []inputStep{
			{prompt: "PlayerController Actor ID", hint: "actor ID with a faction assigned"},
			{prompt: "Scrip delta", hint: "amount to add (negative to subtract)"},
		})
	case viewAwardXP:
		return m.startWizard(viewAwardXP, []inputStep{
			{prompt: "Player ID", hint: "actor ID from Specializations view"},
			{prompt: "Track", hint: "Combat  Crafting  Gathering  Exploration  Sabotage"},
			{prompt: "XP to add", hint: "integer (44182 = max level)"},
		})
	case viewSQL:
		return m.startWizard(viewSQL, []inputStep{
			{prompt: "SQL", hint: "SELECT … (results capped at 200 rows)"},
		})
	}
	return m, nil
}

func (m model) startWizard(target viewState, steps []inputStep) (model, tea.Cmd) {
	m.state = target
	m.inputSteps = steps
	m.inputCursor = 0
	m.textInput.SetValue("")
	m.textInput.Placeholder = steps[0].hint
	m.textInput.Focus()
	return m, textinput.Blink
}

func (m model) isInputState() bool {
	switch m.state {
	case viewGiveItem, viewGiveCurrency, viewGiveFactionRep, viewGiveLandsraadScrip, viewAwardXP, viewSQL, viewInventory:
		return len(m.inputSteps) > 0 && m.inputCursor < len(m.inputSteps)
	}
	return false
}

func (m model) handleInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	switch k {
	case "esc":
		m.inputSteps = nil
		m.inputCursor = 0
		m.state = viewMenu
		return m, nil

	case "tab":
		if m.state == viewGiveItem && m.inputCursor == 1 {
			cur := strings.ToLower(m.textInput.Value())
			for _, t := range itemTemplates {
				if strings.HasPrefix(strings.ToLower(t), cur) {
					m.textInput.SetValue(t)
					break
				}
			}
		}

	case "enter":
		m.inputSteps[m.inputCursor].value = m.textInput.Value()
		if m.inputCursor == len(m.inputSteps)-1 {
			return m.executeWizard()
		}
		m.inputCursor++
		next := m.inputSteps[m.inputCursor]
		m.textInput.Placeholder = next.hint
		if m.state == viewGiveItem && m.inputCursor == 2 {
			m.textInput.SetValue("1")
		} else if m.state == viewGiveItem && m.inputCursor == 3 {
			m.textInput.SetValue("0")
		} else {
			m.textInput.SetValue("")
		}
		return m, textinput.Blink
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) executeWizard() (model, tea.Cmd) {
	vals := m.inputSteps
	m.inputSteps = nil
	m.inputCursor = 0

	parseInt := func(s string, def int64) int64 {
		v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return def
		}
		return v
	}
	parseInt32 := func(s string, def int32) int32 {
		return int32(parseInt(s, int64(def)))
	}

	switch m.state {
	case viewInventory:
		id := parseInt(vals[0].value, 0)
		m.prevState = viewMenu
		return m, cmdFetchInventory(id)

	case viewGiveItem:
		playerID := parseInt(vals[0].value, 0)
		template := strings.TrimSpace(vals[1].value)
		qty := parseInt(vals[2].value, 1)
		quality := parseInt(vals[3].value, 0)
		m.state = viewMenu
		return m, cmdGiveItem(playerID, template, qty, quality)

	case viewGiveCurrency:
		playerID := parseInt(vals[0].value, 0)
		amount := parseInt(vals[1].value, 0)
		m.state = viewMenu
		return m, cmdGiveCurrency(playerID, amount)

	case viewGiveFactionRep:
		actorID := parseInt(vals[0].value, 0)
		factionID := int16(parseInt(vals[1].value, 0))
		delta := parseInt32(vals[2].value, 0)
		m.state = viewMenu
		return m, cmdGiveFactionRep(actorID, factionID, delta)

	case viewGiveLandsraadScrip:
		actorID := parseInt(vals[0].value, 0)
		delta := parseInt32(vals[1].value, 0)
		m.state = viewMenu
		return m, cmdGiveLandsraadScrip(actorID, delta)

	case viewAwardXP:
		playerID := parseInt(vals[0].value, 0)
		track := strings.TrimSpace(vals[1].value)
		delta := parseInt32(vals[2].value, 0)
		m.state = viewMenu
		return m, cmdAwardXP(playerID, track, delta)

	case viewSQL:
		sql := strings.TrimSpace(vals[0].value)
		m.state = viewConnect
		return m, cmdRunSQL(sql)
	}

	m.state = viewMenu
	return m, nil
}

// ── table construction ────────────────────────────────────────────────────────

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

func (m *model) rebuildTable() {
	w, h := m.tableArea()
	s := table.DefaultStyles()
	s.Selected = styleTableSel
	s.Header = styleTableHdr.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(clrOrange).
		BorderBottom(true)

	switch m.state {
	case viewPlayers:
		cols := []table.Column{
			{Title: "ID", Width: 8},
			{Title: "Account", Width: 12},
			{Title: "Class", Width: 20},
			{Title: "Map", Width: 16},
			{Title: "Faction", Width: 12},
		}
		// give leftover width to Class
		fixed := 8 + 12 + 16 + 12 + 4*3
		extra := w - fixed - 2
		if extra > 10 {
			cols[2].Width = extra
		}
		var rows []table.Row
		for _, p := range m.players {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", p.ID),
				fmt.Sprintf("%d", p.AccountID),
				p.Class,
				p.Map,
				factionDisplayName(p.FactionID),
			})
		}
		m.tbl = newTable(cols, rows, w, h, s)

	case viewInventory:
		cols := []table.Column{
			{Title: "ID", Width: 8},
			{Title: "Template", Width: 36},
			{Title: "Qty", Width: 6},
			{Title: "Quality", Width: 8},
			{Title: "Durability", Width: 12},
		}
		extra := w - 8 - 6 - 8 - 12 - 4*3 - 2
		if extra > 10 {
			cols[1].Width = extra
		}
		var rows []table.Row
		for _, it := range m.inventory {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", it.ID),
				it.TemplateID,
				fmt.Sprintf("%d", it.StackSize),
				fmt.Sprintf("%d", it.Quality),
				it.Durability,
			})
		}
		m.tbl = newTable(cols, rows, w, h, s)

	case viewCurrency:
		cols := []table.Column{
			{Title: "Player ID", Width: 14},
			{Title: "Currency", Width: 16},
			{Title: "Balance", Width: 16},
		}
		var rows []table.Row
		for _, c := range m.currencies {
			name := "Solaris"
			if c.CurrencyID != 0 {
				name = fmt.Sprintf("Type %d", c.CurrencyID)
			}
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", c.PlayerID),
				name,
				fmt.Sprintf("%d", c.Balance),
			})
		}
		m.tbl = newTable(cols, rows, w, h, s)

	case viewFactions:
		scripTitle := "Scrips"
		if m.scripCurrencyID > 0 || scripCurrencyID >= 0 {
			id := m.scripCurrencyID
			if id == 0 && scripCurrencyID >= 0 {
				id = int16(scripCurrencyID)
			}
			if id > 0 {
				scripTitle = fmt.Sprintf("Scrips (ID %d)", id)
			}
		}
		cols := []table.Column{
			{Title: "Actor ID", Width: 10},
			{Title: "Faction ID", Width: 10},
			{Title: "Faction", Width: 14},
			{Title: "Reputation", Width: 14},
			{Title: scripTitle, Width: 14},
		}
		var rows []table.Row
		for _, f := range m.factions {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", f.ActorID),
				fmt.Sprintf("%d", f.FactionID),
				f.FactionName,
				fmt.Sprintf("%d", f.Reputation),
				fmt.Sprintf("%d", f.Scrips),
			})
		}
		m.tbl = newTable(cols, rows, w, h, s)

	case viewSpecializations:
		cols := []table.Column{
			{Title: "Player ID", Width: 12},
			{Title: "Track", Width: 14},
			{Title: "XP", Width: 10},
			{Title: "Level", Width: 8},
		}
		var rows []table.Row
		for _, sp := range m.specs {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", sp.PlayerID),
				sp.TrackType,
				fmt.Sprintf("%d", sp.XP),
				fmt.Sprintf("%.1f", sp.Level),
			})
		}
		m.tbl = newTable(cols, rows, w, h, s)

	case viewSQLResult:
		// SQL result is rendered as plain text in the viewport via tbl rows — but
		// we have variable columns, so render as plain text in a single-column table.
		cols := []table.Column{{Title: "Result", Width: w - 2}}
		lines := strings.Split(m.sqlResult, "\n")
		var rows []table.Row
		for _, l := range lines {
			rows = append(rows, table.Row{l})
		}
		m.tbl = newTable(cols, rows, w, h, s)
	}
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
	outerH := m.height

	// ── title bar ─────────────────────────────────────────────────────────────
	connDot := styleDim.Render("○ connecting…")
	if m.connected {
		connDot = styleOK.Render("● " + sshHost)
	}
	titleText := styleTitle.Render("DUNE AWAKENING ADMIN")
	// PlaceHorizontal pads with spaces without clipping ANSI sequences.
	// We place titleText left and connDot right by padding titleText to fill
	// outerW minus the visual width of connDot.
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
	bodyH := outerH - 2 // title row + bottom row
	if bodyH < 4 {
		bodyH = 4
	}

	menuW := 24
	contentW := outerW - menuW - 1 // 1 gap between panes

	menuPane := m.renderMenuPane(menuW, bodyH)
	contentPane := m.renderContentPane(contentW, bodyH)

	body := lipgloss.JoinHorizontal(lipgloss.Top, menuPane, contentPane)

	// ── assemble ──────────────────────────────────────────────────────────────
	var sb strings.Builder
	sb.WriteString(titleBar)
	sb.WriteString("\n")
	sb.WriteString(body)
	sb.WriteString("\n")
	sb.WriteString(bottomLine)
	return sb.String()
}

func (m model) renderMenuPane(w, h int) string {
	inner := h - 2
	innerW := w - 2
	maxLabel := innerW - 2 // room for "▸ " / "  " prefix

	var lines []string
	for i, item := range menuItems {
		if i >= inner {
			break
		}
		label := item.label
		if len([]rune(label)) > maxLabel {
			label = string([]rune(label)[:maxLabel-1]) + "…"
		}
		if i == m.menuCursor && m.state == viewMenu {
			lines = append(lines, styleSelected.Render("▸ "+label))
		} else {
			lines = append(lines, styleNormal.Render("  "+label))
		}
	}
	for len(lines) < inner {
		lines = append(lines, "")
	}
	// Pad each line to innerW individually — safe for pre-styled strings.
	padded := make([]string, len(lines))
	for i, l := range lines {
		padded[i] = lipgloss.PlaceHorizontal(innerW, lipgloss.Left, l)
	}
	body := strings.Join(padded, "\n")

	border := stylePanelBorder
	if m.state == viewMenu {
		border = stylePanelBorderFocused
	}
	rendered := border.Width(w).Height(inner).Render(body)
	return overlayTitle(rendered, " Menu ")
}

func (m model) renderContentPane(w, h int) string {
	inner := h - 2
	innerW := w - 2

	var title, body string

	switch m.state {
	case viewConnect:
		title = " Connecting… "
		body = styleDim.Render("\n  Establishing SSH tunnel to " + sshHost + "…")

	case viewMenu:
		title = " Welcome "
		body = m.renderWelcome(innerW, inner)

	case viewPlayers:
		title = fmt.Sprintf(" Players (%d)  [enter: view inventory] ", len(m.players))
		body = m.tbl.View()

	case viewInventory:
		if len(m.inputSteps) > 0 && m.inputCursor < len(m.inputSteps) {
			step := m.inputSteps[m.inputCursor]
			title = fmt.Sprintf(" %s  [step %d/%d] ", wizardTitle(m.state), m.inputCursor+1, len(m.inputSteps))
			body = m.renderWizardStep(step, innerW, inner)
		} else {
			playerID := int64(0)
			if m.selectedPlayerIdx < len(m.players) {
				playerID = m.players[m.selectedPlayerIdx].ID
			}
			title = fmt.Sprintf(" Inventory — player %d ", playerID)
			body = m.tbl.View()
		}

	case viewCurrency:
		title = fmt.Sprintf(" Currency Balances (%d) ", len(m.currencies))
		body = m.tbl.View()

	case viewFactions:
		title = fmt.Sprintf(" Faction Reputation (%d rows) ", len(m.factions))
		body = m.tbl.View()

	case viewSpecializations:
		title = fmt.Sprintf(" Specialization Tracks (%d rows) ", len(m.specs))
		body = m.tbl.View()

	case viewSQLResult:
		title = " SQL Result "
		body = m.tbl.View()

	case viewGiveItem, viewGiveCurrency, viewGiveFactionRep, viewGiveLandsraadScrip, viewAwardXP, viewSQL:
		if len(m.inputSteps) > 0 && m.inputCursor < len(m.inputSteps) {
			step := m.inputSteps[m.inputCursor]
			title = fmt.Sprintf(" %s  [step %d/%d] ", wizardTitle(m.state), m.inputCursor+1, len(m.inputSteps))
			body = m.renderWizardStep(step, innerW, inner)
		}
	}

	focused := isTableState(m.state)
	border := stylePanelBorder
	if focused {
		border = stylePanelBorderFocused
	}

	// Truncate title if needed (plain string, no ANSI)
	maxTitleW := innerW - 2
	if len([]rune(title)) > maxTitleW {
		title = string([]rune(title)[:maxTitleW])
	}

	// Render the panel border; pass body directly — don't re-wrap in Width/Height
	// which would clip ANSI escape sequences mid-sequence.
	rendered := border.Width(w).Height(inner).Render(body)

	// Overlay the panel title onto the top border
	return overlayTitle(rendered, title)
}

func (m model) renderWelcome(w, h int) string {
	lines := []string{
		"",
		styleOK.Render("  SSH → Kubernetes → PostgreSQL"),
		styleDim.Render("  " + sshHost + "  ▸  cluster pod  ▸  port " + fmt.Sprintf("%d", dbPort)),
		"",
		styleNormal.Render("  Select an action from the menu on the left."),
		"",
		styleDim.Render("  ╭─ Quick reference ─────────────────────────╮"),
		styleDim.Render("  │  Players       · list actors in world      │"),
		styleDim.Render("  │  Inventory     · select player then enter  │"),
		styleDim.Render("  │  Give Item     · inject item directly       │"),
		styleDim.Render("  │  Give Currency · add / subtract Solaris     │"),
		styleDim.Render("  │  Faction Rep   · adjust per-faction scrips  │"),
		styleDim.Render("  │  Landsraad     · add scrip to current house │"),
		styleDim.Render("  │  Award XP      · add XP to any skill track  │"),
		styleDim.Render("  │  SQL           · free-form query            │"),
		styleDim.Render("  ╰────────────────────────────────────────────╯"),
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines[:h], "\n")
}

func (m model) renderWizardStep(step inputStep, w, h int) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styleNormal.Render("  " + step.prompt))
	sb.WriteString("\n")
	sb.WriteString(styleDim.Render("  " + step.hint))
	sb.WriteString("\n\n")
	sb.WriteString("  " + m.textInput.View())

	// show autocomplete hints for item template
	if m.state == viewGiveItem && m.inputCursor == 1 {
		cur := strings.ToLower(m.textInput.Value())
		sb.WriteString("\n\n")
		sb.WriteString(styleDim.Render("  Available (Tab to complete):"))
		sb.WriteString("\n")
		count := 0
		for _, t := range itemTemplates {
			if cur == "" || strings.HasPrefix(strings.ToLower(t), cur) {
				sb.WriteString(styleDim.Render("    · " + t))
				sb.WriteString("\n")
				count++
				if count >= 8 {
					break
				}
			}
		}
	}

	return sb.String()
}

// overlayTitle writes a title string over the top border of a rendered box.
// The border top line is wrapped in ANSI color codes; we strip them, splice
// the title into the plain box-drawing runes, then re-wrap.
func overlayTitle(box, title string) string {
	if title == "" {
		return box
	}
	lines := strings.Split(box, "\n")
	if len(lines) == 0 {
		return box
	}

	raw := lines[0]

	// Extract leading ANSI prefix and trailing reset suffix.
	// Pattern: ESC[ ... m  <box chars>  ESC[m
	ansiPrefix, plainTop, ansiSuffix := splitANSI(raw)

	top := []rune(plainTop)
	t := []rune(" " + strings.TrimSpace(title) + " ")
	if len(t)+2 <= len(top) {
		copy(top[2:], t)
		lines[0] = ansiPrefix + string(top) + ansiSuffix
	}
	return strings.Join(lines, "\n")
}

// splitANSI splits a string of the form "<ANSI>text<reset>" into its three
// parts. Works for the simple single-span case lipgloss produces for borders.
func splitANSI(s string) (prefix, middle, suffix string) {
	// Find end of leading escape sequence(s): sequences end with 'm'
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
			j++ // include the 'm'
		}
		i = j
	}
	prefix = s[:i]
	rest := s[i:]

	// Find trailing reset: "\x1b[m" or "\x1b[0m"
	if idx := strings.LastIndex(rest, "\x1b["); idx >= 0 {
		middle = rest[:idx]
		suffix = rest[idx:]
	} else {
		middle = rest
		suffix = ""
	}
	return
}

// ── helpers ───────────────────────────────────────────────────────────────────

func shortClass(s string) string {
	// strip leading path components
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	s = strings.TrimSuffix(s, "_C")
	// known abbreviations
	replacer := strings.NewReplacer(
		"BP_DunePlayerCharacter", "PlayerCharacter",
		"BP_DunePlayerController", "PlayerController",
		"DunePlayerState", "PlayerState",
	)
	return replacer.Replace(s)
}

func wizardTitle(s viewState) string {
	switch s {
	case viewGiveItem:
		return "Give Item"
	case viewGiveCurrency:
		return "Give Currency"
	case viewGiveFactionRep:
		return "Give Faction Rep"
	case viewGiveLandsraadScrip:
		return "Give Landsraad Scrip"
	case viewAwardXP:
		return "Award XP"
	case viewSQL:
		return "SQL Query"
	case viewInventory:
		return "View Inventory"
	default:
		return "Input"
	}
}

func isTableState(s viewState) bool {
	switch s {
	case viewPlayers, viewInventory, viewCurrency, viewFactions, viewSpecializations, viewSQLResult:
		return true
	}
	return false
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
