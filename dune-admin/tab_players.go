package main

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// playerView tracks which sub-view is active within the Players tab.
type playerView int

const (
	pvMenu playerView = iota
	pvPlayers
	pvInventory
	pvCurrency
	pvFactions
	pvFactionRep
	pvSpecializations
	pvGiveItem
	pvGiveCurrency
	pvGiveFactionRep
	pvGiveLandsraadScrip
	pvAwardXP
	pvSQL
	pvSQLResult
)

// PlayersState holds all state for the Players tab.
type PlayersState struct {
	view       playerView
	prevView   playerView
	menuCursor int

	players         []playerInfo
	inventory       []itemInfo
	currencies      []currencyRow
	factions        []factionRep
	specs           []specTrack
	sqlResult       string
	scripCurrencyID int16

	tbl               table.Model
	selectedPlayerIdx int
	selectedFactionID int16

	inputSteps  []inputStep
	inputCursor int
	textInput   textinput.Model
}

func newPlayersState() PlayersState {
	ti := textinput.New()
	ti.CharLimit = 256
	return PlayersState{textInput: ti}
}

// ── menu items ────────────────────────────────────────────────────────────────

type menuItem struct {
	label string
	view  playerView
}

var menuItems = []menuItem{
	{"Players", pvPlayers},
	{"Inventory", pvInventory},
	{"Currency", pvCurrency},
	{"Factions & Rep", pvFactions},
	{"Specializations / XP", pvSpecializations},
	{"Give Item", pvGiveItem},
	{"Give Currency", pvGiveCurrency},
	{"Give Faction Rep", pvGiveFactionRep},
	{"Give Landsraad Scrip", pvGiveLandsraadScrip},
	{"Award XP", pvAwardXP},
	{"SQL Query", pvSQL},
	{"Quit", pvMenu},
}

var menuQuitIdx = len(menuItems) - 1

var itemTemplates = []string{
	"Stone", "MelangeSpice", "SpiceResidue", "SteelBar", "Oil", "ScrapMetal",
	"T2MachineComponent", "T3MiningGalleryComponent1", "WormTooth",
	"Ammo", "HeavyAmmo", "HealthPack_Channeled_3", "Solaris",
	"PowerPack", "MiningTool_1h_Standard", "BasicBuildingTool",
	"OrnithopterLightEngine_4", "VehicleBackupTool",
}

// ── table construction ────────────────────────────────────────────────────────

func rebuildPlayersTable(m model) model {
	w, h := m.tableArea()
	s := table.DefaultStyles()
	s.Selected = styleTableSel
	s.Header = styleTableHdr.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(clrOrange).
		BorderBottom(true)

	switch m.pl.view {
	case pvPlayers:
		cols := []table.Column{
			{Title: "ID", Width: 8},
			{Title: "Account", Width: 12},
			{Title: "Class", Width: 20},
			{Title: "Map", Width: 16},
			{Title: "Faction", Width: 12},
		}
		fixed := 8 + 12 + 16 + 12 + 4*3
		extra := w - fixed - 2
		if extra > 10 {
			cols[2].Width = extra
		}
		var rows []table.Row
		for _, p := range m.pl.players {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", p.ID),
				fmt.Sprintf("%d", p.AccountID),
				p.Class,
				p.Map,
				factionDisplayName(p.FactionID),
			})
		}
		m.pl.tbl = newTable(cols, rows, w, h, s)

	case pvInventory:
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
		for _, it := range m.pl.inventory {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", it.ID),
				it.TemplateID,
				fmt.Sprintf("%d", it.StackSize),
				fmt.Sprintf("%d", it.Quality),
				it.Durability,
			})
		}
		m.pl.tbl = newTable(cols, rows, w, h, s)

	case pvCurrency:
		cols := []table.Column{
			{Title: "Player ID", Width: 14},
			{Title: "Currency", Width: 16},
			{Title: "Balance", Width: 16},
		}
		var rows []table.Row
		for _, c := range m.pl.currencies {
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
		m.pl.tbl = newTable(cols, rows, w, h, s)

	case pvFactions:
		scripTitle := "Scrips"
		if m.pl.scripCurrencyID > 0 || scripCurrencyID >= 0 {
			id := m.pl.scripCurrencyID
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
		for _, f := range m.pl.factions {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", f.ActorID),
				fmt.Sprintf("%d", f.FactionID),
				f.FactionName,
				fmt.Sprintf("%d", f.Reputation),
				fmt.Sprintf("%d", f.Scrips),
			})
		}
		m.pl.tbl = newTable(cols, rows, w, h, s)

	case pvSpecializations:
		cols := []table.Column{
			{Title: "Player ID", Width: 12},
			{Title: "Track", Width: 14},
			{Title: "XP", Width: 10},
			{Title: "Level", Width: 8},
		}
		var rows []table.Row
		for _, sp := range m.pl.specs {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", sp.PlayerID),
				sp.TrackType,
				fmt.Sprintf("%d", sp.XP),
				fmt.Sprintf("%.1f", sp.Level),
			})
		}
		m.pl.tbl = newTable(cols, rows, w, h, s)

	case pvSQLResult:
		cols := []table.Column{{Title: "Result", Width: w - 2}}
		lines := strings.Split(m.pl.sqlResult, "\n")
		var rows []table.Row
		for _, l := range lines {
			rows = append(rows, table.Row{l})
		}
		m.pl.tbl = newTable(cols, rows, w, h, s)
	}

	return m
}

// ── update ────────────────────────────────────────────────────────────────────

func playersUpdate(msg tea.Msg, m model) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgPlayers:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			m.pl.view = pvMenu
		} else {
			m.pl.players = msg.rows
			m.pl.view = pvPlayers
			m = rebuildPlayersTable(m)
		}
		return m, nil

	case msgInventory:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			m.pl.view = m.pl.prevView
		} else {
			m.pl.inventory = msg.rows
			m.pl.view = pvInventory
			m = rebuildPlayersTable(m)
		}
		return m, nil

	case msgCurrency:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			m.pl.view = pvMenu
		} else {
			m.pl.currencies = msg.rows
			m.pl.view = pvCurrency
			m = rebuildPlayersTable(m)
		}
		return m, nil

	case msgFactions:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			m.pl.view = pvMenu
		} else {
			m.pl.factions = msg.rows
			m.pl.scripCurrencyID = msg.scripCurrencyID
			m.pl.view = pvFactions
			m = rebuildPlayersTable(m)
		}
		return m, nil

	case msgSpecs:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			m.pl.view = pvMenu
		} else {
			m.pl.specs = msg.rows
			m.pl.view = pvSpecializations
			m = rebuildPlayersTable(m)
		}
		return m, nil

	case msgSQL:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			m.pl.view = pvMenu
		} else {
			m.pl.sqlResult = msg.result
			m.pl.view = pvSQLResult
			m = rebuildPlayersTable(m)
		}
		return m, nil

	case msgMutate:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
		} else {
			m.statusMsg, m.statusIsOK = msg.ok, true
		}
		m.pl.view = pvMenu
		return m, nil

	case tea.KeyPressMsg:
		return playersHandleKey(msg, m)
	}
	return m, nil
}

func playersHandleKey(msg tea.KeyPressMsg, m model) (model, tea.Cmd) {
	k := msg.String()

	if k == "q" && m.pl.view == pvMenu {
		return m, tea.Quit
	}

	if playersIsInputState(m) {
		return playersHandleInputKey(msg, m)
	}

	if k == "esc" {
		m.statusMsg = ""
		switch m.pl.view {
		case pvInventory:
			m.pl.view = pvPlayers
			m = rebuildPlayersTable(m)
		default:
			m.pl.view = pvMenu
		}
		return m, nil
	}

	switch m.pl.view {
	case pvMenu:
		return playersHandleMenuKey(k, m)

	case pvPlayers:
		switch k {
		case "enter":
			if len(m.pl.players) > 0 {
				idx := m.pl.tbl.Cursor()
				m.pl.selectedPlayerIdx = idx
				m.pl.prevView = pvPlayers
				return m, cmdFetchInventory(m.pl.players[idx].ID)
			}
		}
		var cmd tea.Cmd
		m.pl.tbl, cmd = m.pl.tbl.Update(msg)
		return m, cmd

	case pvInventory, pvCurrency, pvFactions, pvSpecializations, pvSQLResult:
		var cmd tea.Cmd
		m.pl.tbl, cmd = m.pl.tbl.Update(msg)
		return m, cmd
	}

	return m, nil
}

func playersHandleMenuKey(k string, m model) (model, tea.Cmd) {
	switch k {
	case "up", "k":
		if m.pl.menuCursor > 0 {
			m.pl.menuCursor--
		}
	case "down", "j":
		if m.pl.menuCursor < len(menuItems)-1 {
			m.pl.menuCursor++
		}
	case "enter":
		return playersActivateMenu(m)
	}
	return m, nil
}

func playersActivateMenu(m model) (model, tea.Cmd) {
	m.statusMsg = ""
	idx := m.pl.menuCursor
	if idx == menuQuitIdx {
		return m, tea.Quit
	}
	switch menuItems[idx].view {
	case pvPlayers:
		return m, func() tea.Msg { return cmdFetchPlayers() }
	case pvInventory:
		return playersStartWizard(pvInventory, []inputStep{
			{prompt: "Player ID", hint: "numeric actor ID (see Players view)"},
		}, m)
	case pvCurrency:
		return m, func() tea.Msg { return cmdFetchCurrency() }
	case pvFactions:
		return m, func() tea.Msg { return cmdFetchFactions() }
	case pvSpecializations:
		return m, func() tea.Msg { return cmdFetchSpecs() }
	case pvGiveItem:
		return playersStartWizard(pvGiveItem, []inputStep{
			{prompt: "Player Character ID", hint: "numeric actor ID"},
			{prompt: "Item template", hint: "e.g. MelangeSpice  (Tab to autocomplete)"},
			{prompt: "Quantity", hint: "default: 1"},
			{prompt: "Quality level", hint: "0 = default, 1-4 for higher tier"},
		}, m)
	case pvGiveCurrency:
		return playersStartWizard(pvGiveCurrency, []inputStep{
			{prompt: "Player Controller ID", hint: "numeric actor ID"},
			{prompt: "Amount to add", hint: "Solaris delta (use negative to subtract)"},
		}, m)
	case pvGiveFactionRep:
		return playersStartWizard(pvGiveFactionRep, []inputStep{
			{prompt: "PlayerController Actor ID", hint: "actor ID where faction rep is stored (see Factions view — use the Actor ID column)"},
			{prompt: "Faction ID", hint: "1=Atreides  2=Harkonnen  4=Smuggler"},
			{prompt: "Scrip delta", hint: "amount to add (negative to subtract) — tier tags auto-synced"},
		}, m)
	case pvGiveLandsraadScrip:
		return playersStartWizard(pvGiveLandsraadScrip, []inputStep{
			{prompt: "PlayerController Actor ID", hint: "actor ID with a faction assigned"},
			{prompt: "Scrip delta", hint: "amount to add (negative to subtract)"},
		}, m)
	case pvAwardXP:
		return playersStartWizard(pvAwardXP, []inputStep{
			{prompt: "Player ID", hint: "actor ID from Specializations view"},
			{prompt: "Track", hint: "Combat  Crafting  Gathering  Exploration  Sabotage"},
			{prompt: "XP to add", hint: "integer (44182 = max level)"},
		}, m)
	case pvSQL:
		return playersStartWizard(pvSQL, []inputStep{
			{prompt: "SQL", hint: "SELECT … (results capped at 200 rows)"},
		}, m)
	}
	return m, nil
}

func playersStartWizard(target playerView, steps []inputStep, m model) (model, tea.Cmd) {
	m.pl.view = target
	m.pl.inputSteps = steps
	m.pl.inputCursor = 0
	m.pl.textInput.SetValue("")
	m.pl.textInput.Placeholder = steps[0].hint
	m.pl.textInput.Focus()
	return m, textinput.Blink
}

func playersIsInputState(m model) bool {
	switch m.pl.view {
	case pvGiveItem, pvGiveCurrency, pvGiveFactionRep, pvGiveLandsraadScrip, pvAwardXP, pvSQL, pvInventory:
		return len(m.pl.inputSteps) > 0 && m.pl.inputCursor < len(m.pl.inputSteps)
	}
	return false
}

func playersHandleInputKey(msg tea.KeyPressMsg, m model) (model, tea.Cmd) {
	k := msg.String()
	switch k {
	case "esc":
		m.pl.inputSteps = nil
		m.pl.inputCursor = 0
		m.pl.view = pvMenu
		return m, nil

	case "tab":
		if m.pl.view == pvGiveItem && m.pl.inputCursor == 1 {
			cur := strings.ToLower(m.pl.textInput.Value())
			for _, t := range itemTemplates {
				if strings.HasPrefix(strings.ToLower(t), cur) {
					m.pl.textInput.SetValue(t)
					break
				}
			}
		}

	case "enter":
		m.pl.inputSteps[m.pl.inputCursor].value = m.pl.textInput.Value()
		if m.pl.inputCursor == len(m.pl.inputSteps)-1 {
			return playersExecuteWizard(m)
		}
		m.pl.inputCursor++
		next := m.pl.inputSteps[m.pl.inputCursor]
		m.pl.textInput.Placeholder = next.hint
		if m.pl.view == pvGiveItem && m.pl.inputCursor == 2 {
			m.pl.textInput.SetValue("1")
		} else if m.pl.view == pvGiveItem && m.pl.inputCursor == 3 {
			m.pl.textInput.SetValue("0")
		} else {
			m.pl.textInput.SetValue("")
		}
		return m, textinput.Blink
	}

	var cmd tea.Cmd
	m.pl.textInput, cmd = m.pl.textInput.Update(msg)
	return m, cmd
}

func playersExecuteWizard(m model) (model, tea.Cmd) {
	vals := m.pl.inputSteps
	m.pl.inputSteps = nil
	m.pl.inputCursor = 0

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

	switch m.pl.view {
	case pvInventory:
		id := parseInt(vals[0].value, 0)
		m.pl.prevView = pvMenu
		return m, cmdFetchInventory(id)

	case pvGiveItem:
		playerID := parseInt(vals[0].value, 0)
		template := strings.TrimSpace(vals[1].value)
		qty := parseInt(vals[2].value, 1)
		quality := parseInt(vals[3].value, 0)
		m.pl.view = pvMenu
		return m, cmdGiveItem(playerID, template, qty, quality)

	case pvGiveCurrency:
		playerID := parseInt(vals[0].value, 0)
		amount := parseInt(vals[1].value, 0)
		m.pl.view = pvMenu
		return m, cmdGiveCurrency(playerID, amount)

	case pvGiveFactionRep:
		actorID := parseInt(vals[0].value, 0)
		factionID := int16(parseInt(vals[1].value, 0))
		delta := parseInt32(vals[2].value, 0)
		m.pl.view = pvMenu
		return m, cmdGiveFactionRep(actorID, factionID, delta)

	case pvGiveLandsraadScrip:
		actorID := parseInt(vals[0].value, 0)
		delta := parseInt32(vals[1].value, 0)
		m.pl.view = pvMenu
		return m, cmdGiveLandsraadScrip(actorID, delta)

	case pvAwardXP:
		playerID := parseInt(vals[0].value, 0)
		track := strings.TrimSpace(vals[1].value)
		delta := parseInt32(vals[2].value, 0)
		m.pl.view = pvMenu
		return m, cmdAwardXP(playerID, track, delta)

	case pvSQL:
		sql := strings.TrimSpace(vals[0].value)
		return m, cmdRunSQL(sql)
	}

	m.pl.view = pvMenu
	return m, nil
}

// ── view rendering ────────────────────────────────────────────────────────────

func playersView(m model) string {
	menuW := 24
	contentW := m.width - menuW - 1
	bodyH := m.height - 2
	if bodyH < 4 {
		bodyH = 4
	}

	menuPane := renderPlayersMenuPane(m, menuW, bodyH)
	contentPane := renderPlayersContentPane(m, contentW, bodyH)

	return lipgloss.JoinHorizontal(lipgloss.Top, menuPane, contentPane)
}

func renderPlayersMenuPane(m model, w, h int) string {
	inner := h - 2
	innerW := w - 2
	maxLabel := innerW - 2

	var lines []string
	for i, item := range menuItems {
		if i >= inner {
			break
		}
		label := item.label
		if len([]rune(label)) > maxLabel {
			label = string([]rune(label)[:maxLabel-1]) + "…"
		}
		if i == m.pl.menuCursor && m.pl.view == pvMenu {
			lines = append(lines, styleSelected.Render("▸ "+label))
		} else {
			lines = append(lines, styleNormal.Render("  "+label))
		}
	}
	for len(lines) < inner {
		lines = append(lines, "")
	}
	padded := make([]string, len(lines))
	for i, l := range lines {
		padded[i] = lipgloss.PlaceHorizontal(innerW, lipgloss.Left, l)
	}
	body := strings.Join(padded, "\n")

	border := stylePanelBorder
	if m.pl.view == pvMenu {
		border = stylePanelBorderFocused
	}
	rendered := border.Width(w).Height(inner).Render(body)
	return overlayTitle(rendered, " Menu ")
}

func renderPlayersContentPane(m model, w, h int) string {
	inner := h - 2
	innerW := w - 2

	var title, body string

	switch m.pl.view {
	case pvMenu:
		title = " Welcome "
		body = renderPlayersWelcome(m, innerW, inner)

	case pvPlayers:
		title = fmt.Sprintf(" Players (%d)  [enter: view inventory] ", len(m.pl.players))
		body = m.pl.tbl.View()

	case pvInventory:
		if len(m.pl.inputSteps) > 0 && m.pl.inputCursor < len(m.pl.inputSteps) {
			step := m.pl.inputSteps[m.pl.inputCursor]
			title = fmt.Sprintf(" %s  [step %d/%d] ", wizardTitlePV(m.pl.view), m.pl.inputCursor+1, len(m.pl.inputSteps))
			body = renderPlayersWizardStep(m, step, innerW, inner)
		} else {
			playerID := int64(0)
			if m.pl.selectedPlayerIdx < len(m.pl.players) {
				playerID = m.pl.players[m.pl.selectedPlayerIdx].ID
			}
			title = fmt.Sprintf(" Inventory — player %d ", playerID)
			body = m.pl.tbl.View()
		}

	case pvCurrency:
		title = fmt.Sprintf(" Currency Balances (%d) ", len(m.pl.currencies))
		body = m.pl.tbl.View()

	case pvFactions:
		title = fmt.Sprintf(" Faction Reputation (%d rows) ", len(m.pl.factions))
		body = m.pl.tbl.View()

	case pvSpecializations:
		title = fmt.Sprintf(" Specialization Tracks (%d rows) ", len(m.pl.specs))
		body = m.pl.tbl.View()

	case pvSQLResult:
		title = " SQL Result "
		body = m.pl.tbl.View()

	case pvGiveItem, pvGiveCurrency, pvGiveFactionRep, pvGiveLandsraadScrip, pvAwardXP, pvSQL:
		if len(m.pl.inputSteps) > 0 && m.pl.inputCursor < len(m.pl.inputSteps) {
			step := m.pl.inputSteps[m.pl.inputCursor]
			title = fmt.Sprintf(" %s  [step %d/%d] ", wizardTitlePV(m.pl.view), m.pl.inputCursor+1, len(m.pl.inputSteps))
			body = renderPlayersWizardStep(m, step, innerW, inner)
		}
	}

	focused := isPlayersTableState(m.pl.view)
	border := stylePanelBorder
	if focused {
		border = stylePanelBorderFocused
	}

	maxTitleW := innerW - 2
	if len([]rune(title)) > maxTitleW {
		title = string([]rune(title)[:maxTitleW])
	}

	rendered := border.Width(w).Height(inner).Render(body)
	return overlayTitle(rendered, title)
}

func renderPlayersWelcome(m model, w, h int) string {
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

func renderPlayersWizardStep(m model, step inputStep, w, h int) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styleNormal.Render("  " + step.prompt))
	sb.WriteString("\n")
	sb.WriteString(styleDim.Render("  " + step.hint))
	sb.WriteString("\n\n")
	sb.WriteString("  " + m.pl.textInput.View())

	if m.pl.view == pvGiveItem && m.pl.inputCursor == 1 {
		cur := strings.ToLower(m.pl.textInput.Value())
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

// ── helpers ───────────────────────────────────────────────────────────────────

func shortClass(s string) string {
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	s = strings.TrimSuffix(s, "_C")
	replacer := strings.NewReplacer(
		"BP_DunePlayerCharacter", "PlayerCharacter",
		"BP_DunePlayerController", "PlayerController",
		"DunePlayerState", "PlayerState",
	)
	return replacer.Replace(s)
}

func wizardTitlePV(s playerView) string {
	switch s {
	case pvGiveItem:
		return "Give Item"
	case pvGiveCurrency:
		return "Give Currency"
	case pvGiveFactionRep:
		return "Give Faction Rep"
	case pvGiveLandsraadScrip:
		return "Give Landsraad Scrip"
	case pvAwardXP:
		return "Award XP"
	case pvSQL:
		return "SQL Query"
	case pvInventory:
		return "View Inventory"
	default:
		return "Input"
	}
}

func isPlayersTableState(s playerView) bool {
	switch s {
	case pvPlayers, pvInventory, pvCurrency, pvFactions, pvSpecializations, pvSQLResult:
		return true
	}
	return false
}
