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
	_ // pvFactionRep was removed (unused)
	pvSpecializations
	pvGiveItem
	pvGiveCurrency
	pvGiveFactionRep
	pvGiveLandsraadScrip
	pvSetVitals
	pvSetTechPoints
	pvSetSkillPoints
	pvAddPlayerXP
	pvSetProgression
	pvUnlockAllSkills
	pvAwardXP
	pvSQL
	pvSQLResult
	pvOnlineState
	pvKickPlayer
	pvDeleteItem
	pvResetSpec
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
	onlineState     []onlineStateRow
	structureCounts map[int64]structureCount

	tbl               table.Model
	selectedPlayerIdx int

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
	{"Online State", pvOnlineState},
	{"Give Item", pvGiveItem},
	{"Give Currency", pvGiveCurrency},
	{"Give Faction Rep", pvGiveFactionRep},
	{"Give Landsraad Scrip", pvGiveLandsraadScrip},
	{"Set Health/Hydration", pvSetVitals},
	{"Set Tech Points", pvSetTechPoints},
	{"Set Skill Points", pvSetSkillPoints},
	{"Add Player XP", pvAddPlayerXP},
	{"Set Progression", pvSetProgression},
	{"Unlock All Skills", pvUnlockAllSkills},
	{"Award Spec XP", pvAwardXP},
	{"Kick Player", pvKickPlayer},
	{"Delete Item", pvDeleteItem},
	{"Reset Spec", pvResetSpec},
	{"SQL Query", pvSQL},
	{"Quit", pvMenu},
}

var menuQuitIdx = len(menuItems) - 1

// ── table construction ────────────────────────────────────────────────────────

func rebuildPlayersTable(m model) model {
	w, h := m.tableArea()
	s := table.DefaultStyles()
	s.Selected = styleTableSel
	s.Header = styleTableHdr.Padding(0, 1)
	s.Cell = styleNormal.Padding(0, 1)

	switch m.pl.view {
	case pvPlayers:
		cols := []table.Column{
			{Title: "ID", Width: 8},
			{Title: "Name", Width: 20},
			{Title: "Status", Width: 12},
			{Title: "Class", Width: 18},
			{Title: "Map", Width: 16},
			{Title: "Faction", Width: 10},
			{Title: "Bld/Tot", Width: 9},
		}
		fixed := 8 + 20 + 12 + 18 + 16 + 10 + 9 + 6*3
		extra := w - fixed - 2
		if extra > 0 {
			cols[1].Width += extra / 2
			cols[4].Width += extra - extra/2
		}
		var rows []table.Row
		for _, p := range m.pl.players {
			bldTot := "-"
			if m.pl.structureCounts != nil {
				if sc, ok := m.pl.structureCounts[p.AccountID]; ok {
					bldTot = fmt.Sprintf("%d/%d", sc.Buildings, sc.Totems)
				}
			}
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", p.ID),
				p.Name,
				p.Status,
				p.Class,
				p.Map,
				factionDisplayName(p.FactionID),
				bldTot,
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

	case pvOnlineState:
		cols := []table.Column{
			{Title: "ID", Width: 10},
			{Title: "Name", Width: 20},
			{Title: "Status", Width: 12},
			{Title: "Map", Width: 16},
			{Title: "Last Seen (UTC)", Width: 22},
		}
		var rows []table.Row
		for _, r := range m.pl.onlineState {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", r.PlayerID),
				r.Name,
				r.Status,
				r.Map,
				r.LastSeen,
			})
		}
		m.pl.tbl = newTable(cols, rows, w, h, s)
	}

	return m
}

// ── update ────────────────────────────────────────────────────────────────────

func playersUpdate(msg tea.Msg, m model) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgPlayersBackground:
		if msg.err == nil {
			m.pl.players = msg.rows
			if m.pl.view == pvPlayers {
				m = rebuildPlayersTable(m)
			}
		}
		return m, nil

	case msgOnlineStateBackground:
		if msg.err == nil {
			m.pl.onlineState = msg.rows
			if m.pl.view == pvOnlineState {
				m = rebuildPlayersTable(m)
			}
		}
		return m, nil

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

	case msgInventoryBackground:
		if msg.err == nil {
			m.pl.inventory = msg.rows
			// Rebuild the inventory table so it's ready to display as context in the
			// Delete Item wizard step 2, without switching away from pvDeleteItem.
			prev := m.pl.view
			m.pl.view = pvInventory
			m = rebuildPlayersTable(m)
			m.pl.view = prev
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

	case msgOnlineState:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			m.pl.view = pvMenu
		} else {
			m.pl.onlineState = msg.rows
			m.pl.view = pvOnlineState
			m = rebuildPlayersTable(m)
		}
		return m, nil

	case msgStructures:
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
		} else {
			m.pl.structureCounts = msg.counts
			if m.pl.view == pvPlayers {
				m = rebuildPlayersTable(m)
			}
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

	case pvInventory, pvCurrency, pvFactions, pvSpecializations, pvSQLResult, pvOnlineState:
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
	case "r":
		return m, tea.Batch(tea.Cmd(cmdFetchPlayers), tea.Cmd(cmdFetchStructureCounts))
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
		return m, tea.Batch(tea.Cmd(cmdFetchPlayers), tea.Cmd(cmdFetchStructureCounts))
	case pvInventory:
		return playersStartWizard(pvInventory, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
		}, m)
	case pvCurrency:
		return m, func() tea.Msg { return cmdFetchCurrency() }
	case pvFactions:
		return m, func() tea.Msg { return cmdFetchFactions() }
	case pvSpecializations:
		return m, func() tea.Msg { return cmdFetchSpecs() }
	case pvGiveItem:
		return playersStartWizard(pvGiveItem, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
			{prompt: "Item template", hint: "e.g. MelangeSpice; SolarisCoin is routed to currency"},
			{prompt: "Quantity", hint: "default: 1"},
			{prompt: "Quality level", hint: "0 = default, 1-4 for higher tier"},
		}, m)
	case pvGiveCurrency:
		return playersStartWizard(pvGiveCurrency, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
			{prompt: "Amount to add", hint: "Solaris delta (use negative to subtract)"},
		}, m)
	case pvGiveFactionRep:
		return playersStartWizard(pvGiveFactionRep, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
			{prompt: "Faction ID", hint: "1=Atreides  2=Harkonnen  4=Smuggler"},
			{prompt: "Scrip delta", hint: "amount to add (negative to subtract) — tier tags auto-synced"},
		}, m)
	case pvGiveLandsraadScrip:
		return playersStartWizard(pvGiveLandsraadScrip, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
			{prompt: "Scrip delta", hint: "amount to add (negative to subtract)"},
		}, m)
	case pvSetVitals:
		return playersStartWizard(pvSetVitals, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
			{prompt: "Health", hint: "sets total and current max health"},
			{prompt: "Hydration", hint: "sets base and current hydration"},
		}, m)
	case pvSetTechPoints:
		return playersStartWizard(pvSetTechPoints, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
			{prompt: "Tech points", hint: "sets unspent tech points only"},
		}, m)
	case pvSetSkillPoints:
		return playersStartWizard(pvSetSkillPoints, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
			{prompt: "Skill points", hint: "sets FLevelComponent UnspentSkillPoints"},
		}, m)
	case pvAddPlayerXP:
		return playersStartWizard(pvAddPlayerXP, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
			{prompt: "XP to add", hint: "adds to FLevelComponent TotalXPEarned"},
		}, m)
	case pvSetProgression:
		return playersStartWizard(pvSetProgression, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
			{prompt: "Total XP", hint: "sets FLevelComponent TotalXPEarned"},
			{prompt: "Total skill points", hint: "sets FLevelComponent TotalSkillPoints"},
			{prompt: "Unspent skill points", hint: "sets FLevelComponent UnspentSkillPoints"},
			{prompt: "Tech points", hint: "sets TechKnowledgePlayerComponent points"},
		}, m)
	case pvUnlockAllSkills:
		return playersStartWizard(pvUnlockAllSkills, []inputStep{
			{prompt: "Player name", hint: "sets all FLevelComponent ModuleData SkillPointsSpent to at least 1"},
		}, m)
	case pvAwardXP:
		return playersStartWizard(pvAwardXP, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete"},
			{prompt: "Track", hint: "Combat  Crafting  Gathering  Exploration  Sabotage"},
			{prompt: "XP to add", hint: "integer (44182 = max level)"},
		}, m)
	case pvSQL:
		return playersStartWizard(pvSQL, []inputStep{
			{prompt: "SQL", hint: "SELECT / UPDATE / INSERT / DELETE … (SELECT results capped at 200 rows)"},
		}, m)
	case pvOnlineState:
		return m, func() tea.Msg { return cmdFetchOnlineState() }
	case pvKickPlayer:
		return playersStartWizard(pvKickPlayer, []inputStep{
			{prompt: "Player name (or actor ID)", hint: "type name, Tab to autocomplete — sets status to LoggingOut, no data deleted"},
		}, m)
	case pvDeleteItem:
		return playersStartWizard(pvDeleteItem, []inputStep{
			{prompt: "Player name", hint: "type name, Tab to autocomplete — loads their inventory"},
			{prompt: "Item ID", hint: "numeric item ID shown in the inventory table above"},
		}, m)
	case pvResetSpec:
		return playersStartWizard(pvResetSpec, []inputStep{
			{prompt: "Player name (or actor ID)", hint: "type name, Tab to autocomplete"},
			{prompt: "Track to reset", hint: "Combat  Crafting  Gathering  Exploration  Sabotage  all"},
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
	case pvGiveItem, pvGiveCurrency, pvGiveFactionRep, pvGiveLandsraadScrip, pvAwardXP, pvSQL, pvInventory,
		pvKickPlayer, pvDeleteItem, pvResetSpec, pvSetVitals, pvSetTechPoints, pvSetSkillPoints, pvAddPlayerXP, pvSetProgression, pvUnlockAllSkills:
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
			if matches := itemSuggestions(cur, 1); len(matches) > 0 {
				m.pl.textInput.SetValue(matches[0])
			}
		} else if m.pl.inputCursor == 0 {
			cur := strings.ToLower(m.pl.textInput.Value())
			for _, p := range m.pl.players {
				if p.Name != "" && (cur == "" || strings.HasPrefix(strings.ToLower(p.Name), cur)) {
					m.pl.textInput.SetValue(p.Name)
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
		// For Delete Item step 0→1: fetch the player's inventory so it's
		// visible as context while the admin types the item ID.
		if m.pl.view == pvDeleteItem && m.pl.inputCursor == 1 {
			playerID := lookupPawnIDFromPlayers(m.pl.players, m.pl.inputSteps[0].value)
			if playerID > 0 {
				return m, tea.Batch(textinput.Blink, cmdFetchInventoryBackground(playerID))
			}
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
	parseFloat := func(s string, def float64) float64 {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return def
		}
		return v
	}

	// Resolve a player name to their pawn ID (for inventory/items/XP).
	lookupPawnID := func(name string) int64 {
		name = strings.TrimSpace(name)
		for _, p := range m.pl.players {
			if strings.EqualFold(p.Name, name) {
				return p.ID
			}
		}
		return parseInt(name, 0) // fallback: treat as raw ID
	}

	// Resolve a player name to their controller ID (for currency/faction/scrip).
	lookupControllerID := func(name string) int64 {
		name = strings.TrimSpace(name)
		for _, p := range m.pl.players {
			if strings.EqualFold(p.Name, name) {
				return p.ControllerID
			}
		}
		return parseInt(name, 0) // fallback: treat as raw ID
	}

	switch m.pl.view {
	case pvInventory:
		id := lookupPawnID(vals[0].value)
		m.pl.prevView = pvMenu
		return m, cmdFetchInventory(id)

	case pvGiveItem:
		playerID := lookupPawnID(vals[0].value)
		template := strings.TrimSpace(vals[1].value)
		qty := parseInt(vals[2].value, 1)
		quality := parseInt(vals[3].value, 0)
		m.pl.view = pvMenu
		return m, cmdGiveItem(playerID, template, qty, quality)

	case pvGiveCurrency:
		controllerID := lookupControllerID(vals[0].value)
		amount := parseInt(vals[1].value, 0)
		m.pl.view = pvMenu
		return m, cmdGiveCurrency(controllerID, amount)

	case pvGiveFactionRep:
		controllerID := lookupControllerID(vals[0].value)
		factionID := int16(parseInt(vals[1].value, 0))
		delta := parseInt32(vals[2].value, 0)
		m.pl.view = pvMenu
		return m, cmdGiveFactionRep(controllerID, factionID, delta)

	case pvGiveLandsraadScrip:
		controllerID := lookupControllerID(vals[0].value)
		delta := parseInt32(vals[1].value, 0)
		m.pl.view = pvMenu
		return m, cmdGiveLandsraadScrip(controllerID, delta)

	case pvSetVitals:
		playerID := lookupPawnID(vals[0].value)
		health := parseFloat(vals[1].value, 0)
		hydration := parseFloat(vals[2].value, 0)
		m.pl.view = pvMenu
		return m, cmdSetPlayerVitals(playerID, health, hydration)

	case pvSetTechPoints:
		playerID := lookupPawnID(vals[0].value)
		points := parseInt(vals[1].value, 0)
		m.pl.view = pvMenu
		return m, cmdSetTechPoints(playerID, points)

	case pvSetSkillPoints:
		playerID := lookupPawnID(vals[0].value)
		points := parseInt(vals[1].value, 0)
		m.pl.view = pvMenu
		return m, cmdSetSkillPoints(playerID, points)

	case pvAddPlayerXP:
		playerID := lookupPawnID(vals[0].value)
		delta := parseInt(vals[1].value, 0)
		m.pl.view = pvMenu
		return m, cmdAddPlayerXP(playerID, delta)

	case pvSetProgression:
		playerID := lookupPawnID(vals[0].value)
		totalXP := parseInt(vals[1].value, 0)
		totalSkill := parseInt(vals[2].value, 0)
		unspentSkill := parseInt(vals[3].value, 0)
		techPoints := parseInt(vals[4].value, 0)
		m.pl.view = pvMenu
		return m, cmdSetProgression(playerID, totalXP, totalSkill, unspentSkill, techPoints)

	case pvUnlockAllSkills:
		playerID := lookupPawnID(vals[0].value)
		m.pl.view = pvMenu
		return m, cmdUnlockAllSkills(playerID)

	case pvAwardXP:
		playerID := lookupPawnID(vals[0].value)
		track := strings.TrimSpace(vals[1].value)
		delta := parseInt32(vals[2].value, 0)
		m.pl.view = pvMenu
		return m, cmdAwardXP(playerID, track, delta)

	case pvSQL:
		sql := strings.TrimSpace(vals[0].value)
		return m, cmdRunSQL(sql)

	case pvKickPlayer:
		playerID := lookupPawnID(vals[0].value)
		m.pl.view = pvMenu
		return m, cmdKickPlayer(playerID)

	case pvDeleteItem:
		itemID := parseInt(vals[1].value, 0)
		m.pl.view = pvMenu
		return m, cmdDeleteItem(itemID)

	case pvResetSpec:
		playerID := lookupPawnID(vals[0].value)
		trackType := strings.TrimSpace(vals[1].value)
		m.pl.view = pvMenu
		return m, cmdResetSpecializations(playerID, trackType)
	}

	m.pl.view = pvMenu
	return m, nil
}

// ── view rendering ────────────────────────────────────────────────────────────

func playersView(m model) string {
	menuW := adaptiveMenuWidth(m.width)
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

	case pvOnlineState:
		title = fmt.Sprintf(" Online State (%d players) ", len(m.pl.onlineState))
		body = m.pl.tbl.View()

	case pvGiveItem, pvGiveCurrency, pvGiveFactionRep, pvGiveLandsraadScrip, pvSetVitals, pvSetTechPoints, pvSetSkillPoints, pvAddPlayerXP, pvSetProgression, pvUnlockAllSkills, pvAwardXP, pvSQL,
		pvKickPlayer, pvResetSpec:
		if len(m.pl.inputSteps) > 0 && m.pl.inputCursor < len(m.pl.inputSteps) {
			step := m.pl.inputSteps[m.pl.inputCursor]
			title = fmt.Sprintf(" %s  [step %d/%d] ", wizardTitlePV(m.pl.view), m.pl.inputCursor+1, len(m.pl.inputSteps))
			body = renderPlayersWizardStep(m, step, innerW, inner)
		}

	case pvDeleteItem:
		if len(m.pl.inputSteps) > 0 && m.pl.inputCursor < len(m.pl.inputSteps) {
			step := m.pl.inputSteps[m.pl.inputCursor]
			title = fmt.Sprintf(" Delete Item  [step %d/%d] ", m.pl.inputCursor+1, len(m.pl.inputSteps))
			if m.pl.inputCursor == 1 && len(m.pl.inventory) > 0 {
				// Step 2: show inventory table as context, wizard input below.
				tblLines := strings.Split(m.pl.tbl.View(), "\n")
				maxTblLines := inner/2 - 1
				if len(tblLines) > maxTblLines {
					tblLines = tblLines[:maxTblLines]
				}
				wizBody := renderPlayersWizardStep(m, step, innerW, inner-len(tblLines)-1)
				body = strings.Join(tblLines, "\n") + "\n" + wizBody
			} else {
				body = renderPlayersWizardStep(m, step, innerW, inner)
			}
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
	boxW := 58
	if w < boxW+4 {
		boxW = w - 4
	}
	if boxW < 42 {
		boxW = 42
	}
	row := func(label, desc string) string {
		textW := boxW - 4
		line := fmt.Sprintf("%-15s · %s", label, desc)
		if len([]rune(line)) > textW {
			line = string([]rune(line)[:textW-1]) + "…"
		}
		return styleDim.Render("  │ " + fmt.Sprintf("%-*s", textW, line) + " │")
	}
	lines := []string{
		"",
		styleOK.Render("  AMP local DB → PostgreSQL"),
		styleDim.Render("  127.0.0.1  ▸  port " + fmt.Sprintf("%d", dbPort)),
		"",
		styleNormal.Render("  Select an action from the menu on the left."),
		styleHelp.Render("  Tip: many live values are cached by the server; edit offline, then rejoin."),
		"",
		styleDim.Render("  ╭" + strings.Repeat("─", boxW-2) + "╮"),
		row("Players", "list actors in world"),
		row("Inventory", "select player, Enter to open"),
		row("Give Item", "inject item directly"),
		row("Currency", "add/subtract Solaris"),
		row("Health/Hyd.", "set max health and water"),
		row("Tech Points", "set unspent research points"),
		row("Progression", "set XP, skill and tech points"),
		row("Unlock Skills", "set modules learned"),
		row("Spec XP", "add specialization-track XP"),
		row("Kick Player", "sets LoggingOut if DB row exists"),
		row("SQL", "free-form query / update"),
		styleDim.Render("  ╰" + strings.Repeat("─", boxW-2) + "╯"),
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines[:h], "\n")
}

func requiresOfflineReload(v playerView) bool {
	switch v {
	case pvGiveItem, pvDeleteItem, pvSetVitals, pvSetTechPoints, pvSetSkillPoints, pvAddPlayerXP, pvSetProgression, pvUnlockAllSkills, pvAwardXP, pvResetSpec:
		return true
	default:
		return false
	}
}

func renderPlayersWizardStep(m model, step inputStep, w, h int) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styleNormal.Render("  " + step.prompt))
	sb.WriteString("\n")
	sb.WriteString(styleDim.Render("  " + step.hint))
	sb.WriteString("\n\n")
	input := m.pl.textInput
	if w > 10 {
		input.SetWidth(w - 6)
	}
	sb.WriteString("  " + input.View())

	if requiresOfflineReload(m.pl.view) {
		sb.WriteString("\n")
		sb.WriteString(styleHelp.Render("  Note: server caches this while online — leave/kick, edit, then rejoin."))
	}

	cur := strings.ToLower(m.pl.textInput.Value())
	if m.pl.inputCursor == 0 {
		// Player name suggestions on every wizard step 0.
		sb.WriteString("\n\n")
		sb.WriteString(styleDim.Render("  Players (Tab to complete):"))
		sb.WriteString("\n")
		count := 0
		for _, p := range m.pl.players {
			if p.Name != "" && (cur == "" || strings.HasPrefix(strings.ToLower(p.Name), cur)) {
				sb.WriteString(styleDim.Render("    · " + p.Name))
				sb.WriteString("\n")
				count++
				if count >= 20 {
					break
				}
			}
		}
		if len(m.pl.players) == 0 {
			sb.WriteString(styleDim.Render("    (loading…)"))
			sb.WriteString("\n")
		}
	} else if m.pl.view == pvGiveItem && m.pl.inputCursor == 1 {
		sb.WriteString("\n\n")
		sb.WriteString(styleDim.Render("  Items (Tab to complete, searches template & name):"))
		sb.WriteString("\n")
		matches := itemSuggestions(cur, 20)
		for _, t := range matches {
			sb.WriteString(styleDim.Render("    · " + itemDisplayLine(t)))
			sb.WriteString("\n")
		}
		if len(dbItemTemplates) == 0 && itemData.Items == nil {
			sb.WriteString(styleDim.Render("    (loading…)"))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ── item search helpers ───────────────────────────────────────────────────────

// lookupPawnIDFromPlayers resolves a player name (or raw ID string) to a pawn actor ID.
func lookupPawnIDFromPlayers(players []playerInfo, name string) int64 {
	name = strings.TrimSpace(name)
	for _, p := range players {
		if strings.EqualFold(p.Name, name) {
			return p.ID
		}
	}
	v, _ := strconv.ParseInt(name, 10, 64)
	return v
}

// cmdFetchInventoryBackground fetches inventory for a player without changing the
// active view — used so Delete Item can show the inventory table as context on step 2.
func cmdFetchInventoryBackground(playerID int64) tea.Cmd {
	return func() tea.Msg {
		msg := cmdFetchInventory(playerID)()
		if inv, ok := msg.(msgInventory); ok {
			return msgInventoryBackground(inv)
		}
		return msg
	}
}

// msgInventoryBackground wraps a msgInventory so the update handler can populate
// m.pl.inventory without switching views.
type msgInventoryBackground msgInventory

// itemSuggestions returns up to n template_ids whose template prefix OR display
// name contains cur (case-insensitive). Template-prefix matches come first.
func itemSuggestions(cur string, n int) []string {
	seen := make(map[string]bool)
	var out []string

	add := func(t string) bool {
		if seen[strings.ToLower(t)] {
			return false
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
		return len(out) < n
	}

	// Pass 1: template prefix match (already sorted, PascalCase from DB first).
	for _, t := range dbItemTemplates {
		if cur == "" || strings.HasPrefix(strings.ToLower(t), cur) {
			if !add(t) {
				return out
			}
		}
	}

	// Pass 2: name substring match against duneItemNames (authoritative names + PascalCase IDs).
	for _, entry := range duneItemNames {
		if cur != "" && !strings.Contains(strings.ToLower(entry.Name), cur) {
			continue
		}
		if !add(entry.ID) {
			return out
		}
	}

	// Pass 3: name substring match against itemData for items not in duneItemNames.
	if itemData.Items != nil {
		for k, rule := range itemData.Items {
			if rule.Name == "" {
				continue
			}
			if cur != "" && !strings.Contains(strings.ToLower(rule.Name), cur) {
				continue
			}
			template := rule.TemplateID
			if template == "" {
				template = k
			}
			for _, t := range dbItemTemplates {
				if strings.EqualFold(t, template) {
					template = t
					break
				}
			}
			if !add(template) {
				return out
			}
		}
	}
	return out
}

// itemDisplayLine returns "template  Name" when a display name is known,
// preferring dune-item-names.json over item-data.json.
func itemDisplayLine(template string) string {
	key := strings.ToLower(template)
	if entry, ok := duneItemNames[key]; ok && entry.Name != "" {
		return fmt.Sprintf("%-40s  %s", template, entry.Name)
	}
	if itemData.Items != nil {
		if rule, ok := itemData.Items[key]; ok && rule.Name != "" {
			return fmt.Sprintf("%-40s  %s", template, rule.Name)
		}
	}
	return template
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
	case pvSetVitals:
		return "Set Health/Hydration"
	case pvSetTechPoints:
		return "Set Tech Points"
	case pvSetSkillPoints:
		return "Set Skill Points"
	case pvAddPlayerXP:
		return "Add Player XP"
	case pvSetProgression:
		return "Set Progression"
	case pvUnlockAllSkills:
		return "Unlock All Skills"
	case pvAwardXP:
		return "Award XP"
	case pvSQL:
		return "SQL Query"
	case pvInventory:
		return "View Inventory"
	case pvKickPlayer:
		return "Kick Player"
	case pvDeleteItem:
		return "Delete Item"
	case pvResetSpec:
		return "Reset Specialization"
	default:
		return "Input"
	}
}

func isPlayersTableState(s playerView) bool {
	switch s {
	case pvPlayers, pvInventory, pvCurrency, pvFactions, pvSpecializations, pvSQLResult, pvOnlineState:
		return true
	}
	return false
}
