package main

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type dbSubView int

const (
	dbvMenu dbSubView = iota
	dbvTables
	dbvResult  // shows describe / sample / search / SQL output
	dbvSearch  // text input for column search
	dbvSQL     // text input for raw SQL
)

// DatabaseState holds all state for the Database tab.
type DatabaseState struct {
	menu          int
	subView       dbSubView
	tbl           table.Model
	tables        []tableRow
	selectedTable string
	searchInput   textinput.Model
	sqlInput      textinput.Model
	result        string
	loading       bool
}

var dbMenuLabels = []string{
	"Tables", "Describe", "Sample", "Search Columns", "Run SQL",
}

func newDatabaseState() DatabaseState {
	si := textinput.New()
	si.Placeholder = "table or column name…"
	si.CharLimit = 128

	qi := textinput.New()
	qi.Placeholder = "SELECT …"
	qi.CharLimit = 512

	return DatabaseState{searchInput: si, sqlInput: qi}
}

// databaseUpdate handles all messages for the Database tab.
func databaseUpdate(msg tea.Msg, m model) (model, tea.Cmd) {
	db := &m.db

	switch msg := msg.(type) {
	case msgTables:
		db.loading = false
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
		} else {
			db.tables = msg.rows
			db.tbl = buildTablesWidget(msg.rows, m.width)
			db.subView = dbvTables
		}
		return m, nil

	case msgDescribe:
		db.loading = false
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			db.subView = dbvMenu
		} else {
			db.result = renderDescribeResult(msg.table, msg.cols)
			db.subView = dbvResult
		}
		return m, nil

	case msgSample:
		db.loading = false
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			db.subView = dbvMenu
		} else {
			db.result = renderGridResult(msg.table+" (sample)", msg.headers, msg.rows)
			db.subView = dbvResult
		}
		return m, nil

	case msgSearchCols:
		db.loading = false
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			db.subView = dbvMenu
		} else {
			db.result = renderGridResult("Column search", msg.headers, msg.rows)
			db.subView = dbvResult
		}
		return m, nil

	case msgSQL:
		db.loading = false
		if msg.err != nil {
			m.statusMsg, m.statusIsOK = msg.err.Error(), false
			db.subView = dbvMenu
		} else {
			db.result = msg.result
			db.subView = dbvResult
		}
		return m, nil

	case tea.KeyPressMsg:
		k := msg.String()

		switch db.subView {
		case dbvSearch:
			switch k {
			case "enter":
				if db.searchInput.Value() != "" {
					db.loading = true
					db.subView = dbvMenu
					return m, cmdSearchColumns(db.searchInput.Value())
				}
			case "esc":
				db.subView = dbvMenu
				db.searchInput.SetValue("")
			default:
				var cmd tea.Cmd
				db.searchInput, cmd = db.searchInput.Update(msg)
				return m, cmd
			}
			return m, nil

		case dbvSQL:
			switch k {
			case "enter":
				if db.sqlInput.Value() != "" {
					db.loading = true
					db.subView = dbvMenu
					return m, cmdRunSQL(db.sqlInput.Value())
				}
			case "esc":
				db.subView = dbvMenu
				db.sqlInput.SetValue("")
			default:
				var cmd tea.Cmd
				db.sqlInput, cmd = db.sqlInput.Update(msg)
				return m, cmd
			}
			return m, nil

		case dbvTables:
			switch k {
			case "enter":
				row := db.tbl.SelectedRow()
				if len(row) > 0 {
					db.selectedTable = row[0]
					db.loading = true
					return m, cmdSampleTable(db.selectedTable, 20)
				}
			case "d":
				row := db.tbl.SelectedRow()
				if len(row) > 0 {
					db.loading = true
					return m, cmdDescribeTable(row[0])
				}
			case "esc":
				db.subView = dbvMenu
			default:
				var cmd tea.Cmd
				db.tbl, cmd = db.tbl.Update(msg)
				return m, cmd
			}
			return m, nil

		case dbvResult:
			if k == "esc" {
				db.subView = dbvMenu
				db.result = ""
			}
			return m, nil

		default: // dbvMenu
			switch k {
			case "up", "k":
				if db.menu > 0 {
					db.menu--
				}
			case "down", "j":
				if db.menu < len(dbMenuLabels)-1 {
					db.menu++
				}
			case "enter":
				switch db.menu {
				case 0: // Tables
					db.loading = true
					return m, tea.Cmd(cmdFetchTables)
				case 1: // Describe — load table list first, then press d
					db.loading = true
					return m, tea.Cmd(cmdFetchTables)
				case 2: // Sample — load table list first, then press Enter
					db.loading = true
					return m, tea.Cmd(cmdFetchTables)
				case 3: // Search Columns
					db.subView = dbvSearch
					db.searchInput.SetValue("")
					return m, db.searchInput.Focus()
				case 4: // Run SQL
					db.subView = dbvSQL
					db.sqlInput.SetValue("")
					return m, db.sqlInput.Focus()
				}
			case "esc":
				db.subView = dbvMenu
				db.result = ""
			}
		}
	}
	return m, nil
}

// buildTablesWidget creates a bubbles table widget from the table list.
func buildTablesWidget(rows []tableRow, width int) table.Model {
	nameW := 42
	rowW := 14
	cols := []table.Column{
		{Title: "Table", Width: nameW},
		{Title: "Rows", Width: rowW},
	}
	var trows []table.Row
	for _, r := range rows {
		trows = append(trows, table.Row{r.Name, fmt.Sprintf("%d", r.RowCount)})
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(trows),
		table.WithFocused(true),
		table.WithHeight(20),
	)
	s := table.DefaultStyles()
	s.Header = styleTableHdr
	s.Selected = styleTableSel
	t.SetStyles(s)
	return t
}

// renderDescribeResult formats column info for display.
func renderDescribeResult(tbl string, cols []columnInfo) string {
	var sb strings.Builder
	sb.WriteString(styleTitle.Render("  "+tbl) + "\n\n")
	sb.WriteString(fmt.Sprintf("  %-32s %-26s %s\n", "COLUMN", "TYPE", "NULLABLE"))
	sb.WriteString("  " + strings.Repeat("─", 70) + "\n")
	for _, c := range cols {
		sb.WriteString(fmt.Sprintf("  %-32s %-26s %s\n", c.Name, c.DataType, c.Nullable))
	}
	return sb.String()
}

// renderGridResult formats a header+rows result for display.
func renderGridResult(title string, headers []string, rows [][]string) string {
	var sb strings.Builder
	if title != "" {
		sb.WriteString(styleTitle.Render("  "+title) + "\n\n")
	}
	if len(headers) > 0 {
		sb.WriteString("  " + strings.Join(headers, "  │  ") + "\n")
		sb.WriteString("  " + strings.Repeat("─", 60) + "\n")
	}
	for _, row := range rows {
		sb.WriteString("  " + strings.Join(row, "  │  ") + "\n")
	}
	if len(rows) == 0 {
		sb.WriteString(styleDim.Render("  (no results)") + "\n")
	}
	return sb.String()
}

// databaseView renders the Database tab.
func databaseView(m model) string {
	db := m.db
	menuW := 22
	contentW := m.width - menuW - 5
	if contentW < 10 {
		contentW = 10
	}

	// Left menu pane
	var menuLines []string
	for i, label := range dbMenuLabels {
		hint := ""
		switch i {
		case 1:
			hint = " (d key)"
		case 2:
			hint = " (Enter)"
		}
		line := "  " + label + hint
		if i == db.menu {
			line = styleSelected.Render("▶ " + label + hint)
		} else {
			line = styleDim.Render(line)
		}
		menuLines = append(menuLines, line)
	}
	menuPane := stylePanelBorder.Width(menuW).Render(strings.Join(menuLines, "\n"))

	// Right content pane
	var body string
	switch {
	case db.loading:
		body = styleDim.Render("  loading…")
	case db.subView == dbvSearch:
		body = styleTitle.Render("  Search Columns") +
			"\n\n  " + db.searchInput.View() +
			"\n\n" + styleHelp.Render("  Enter=search   Esc=cancel")
	case db.subView == dbvSQL:
		body = styleTitle.Render("  SQL Query") +
			"\n\n  " + db.sqlInput.View() +
			"\n\n" + styleHelp.Render("  Enter=run   Esc=cancel")
	case db.subView == dbvTables:
		body = db.tbl.View() +
			"\n" + styleHelp.Render("  Enter=sample   d=describe   Esc=back")
	case db.subView == dbvResult:
		body = db.result + "\n" + styleHelp.Render("  Esc=back")
	default:
		body = styleDim.Render("  Select an option and press Enter.\n\n") +
			styleHelp.Render("  Tables/Describe/Sample all open the table list.\n  Use Enter or d after selecting a table.")
	}

	contentPane := stylePanelBorderFocused.Width(contentW).Render(body)
	help := styleHelp.Render("  ↑↓ navigate   Enter select   Esc back")
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, menuPane, contentPane),
		help,
	)
}
