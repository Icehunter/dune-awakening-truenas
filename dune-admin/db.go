package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

// ── data fetch commands ───────────────────────────────────────────────────────

func cmdFetchPlayers() tea.Msg {
	if globalDB == nil {
		return msgPlayers{err: fmt.Errorf("not connected")}
	}
	rows, err := globalDB.Query(context.Background(), `
		SELECT a.id,
		       COALESCE(a.owner_account_id, 0),
		       a.class,
		       COALESCE(a.map, ''),
		       COALESCE(pf.faction_id, 0)
		FROM dune.actors a
		LEFT JOIN dune.player_faction pf ON pf.actor_id = a.id
		WHERE a.class ILIKE '%PlayerCharacter%'
		ORDER BY a.id`)
	if err != nil {
		return msgPlayers{err: err}
	}
	defer rows.Close()

	var players []playerInfo
	for rows.Next() {
		var p playerInfo
		if err := rows.Scan(&p.ID, &p.AccountID, &p.Class, &p.Map, &p.FactionID); err != nil {
			continue
		}
		p.Class = shortClass(p.Class)
		players = append(players, p)
	}
	if rows.Err() != nil {
		return msgPlayers{err: rows.Err()}
	}
	return msgPlayers{rows: players}
}

func cmdFetchInventory(playerID int64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgInventory{err: fmt.Errorf("not connected")}
		}
		rows, err := globalDB.Query(context.Background(), `
			SELECT i.id, i.template_id, i.stack_size, i.quality_level,
			       COALESCE((i.stats->'FItemStackAndDurabilityStats'->1->>'CurrentDurability'), 'N/A')
			FROM dune.items i
			JOIN dune.inventories inv ON i.inventory_id = inv.id
			WHERE inv.actor_id = $1
			ORDER BY i.template_id`, playerID)
		if err != nil {
			return msgInventory{err: err}
		}
		defer rows.Close()

		var items []itemInfo
		for rows.Next() {
			var it itemInfo
			if err := rows.Scan(&it.ID, &it.TemplateID, &it.StackSize, &it.Quality, &it.Durability); err != nil {
				continue
			}
			items = append(items, it)
		}
		if err := rows.Err(); err != nil {
			return msgInventory{err: err}
		}
		return msgInventory{rows: items}
	}
}

func cmdFetchCurrency() tea.Msg {
	if globalDB == nil {
		return msgCurrency{err: fmt.Errorf("not connected")}
	}
	rows, err := globalDB.Query(context.Background(), `
		SELECT player_controller_id, currency_id, balance
		FROM dune.player_virtual_currency_balances
		ORDER BY player_controller_id, currency_id`)
	if err != nil {
		return msgCurrency{err: err}
	}
	defer rows.Close()

	var out []currencyRow
	for rows.Next() {
		var r currencyRow
		if err := rows.Scan(&r.PlayerID, &r.CurrencyID, &r.Balance); err != nil {
			continue
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return msgCurrency{err: err}
	}
	return msgCurrency{rows: out}
}

func cmdFetchFactions() tea.Msg {
	if globalDB == nil {
		return msgFactions{err: fmt.Errorf("not connected")}
	}
	ctx := context.Background()
	scripID, err := resolveScripCurrencyID(ctx)
	if err != nil {
		return msgFactions{err: err}
	}
	rows, err := globalDB.Query(ctx, `
		SELECT pfr.actor_id, pfr.faction_id, f.name, pfr.reputation_amount,
		       COALESCE(vcb.balance, 0)
		FROM dune.player_faction_reputation pfr
		JOIN dune.factions f ON f.id = pfr.faction_id
		LEFT JOIN dune.player_virtual_currency_balances vcb
			ON vcb.player_controller_id = pfr.actor_id
			AND vcb.currency_id = $1
		ORDER BY pfr.actor_id, pfr.faction_id`, scripID)
	if err != nil {
		return msgFactions{err: err}
	}
	defer rows.Close()

	var out []factionRep
	for rows.Next() {
		var r factionRep
		if err := rows.Scan(&r.ActorID, &r.FactionID, &r.FactionName, &r.Reputation, &r.Scrips); err != nil {
			continue
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return msgFactions{err: err}
	}
	return msgFactions{rows: out, scripCurrencyID: scripID}
}

func cmdFetchSpecs() tea.Msg {
	if globalDB == nil {
		return msgSpecs{err: fmt.Errorf("not connected")}
	}
	rows, err := globalDB.Query(context.Background(), `
		SELECT player_id, track_type::text, xp_amount, level
		FROM dune.specialization_tracks
		ORDER BY player_id, track_type`)
	if err != nil {
		return msgSpecs{err: err}
	}
	defer rows.Close()

	var out []specTrack
	for rows.Next() {
		var r specTrack
		if err := rows.Scan(&r.PlayerID, &r.TrackType, &r.XP, &r.Level); err != nil {
			continue
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return msgSpecs{err: err}
	}
	return msgSpecs{rows: out}
}

func cmdRunSQL(sql string) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgSQL{err: fmt.Errorf("not connected")}
		}
		rows, err := globalDB.Query(context.Background(), sql)
		if err != nil {
			return msgSQL{err: err}
		}
		defer rows.Close()

		var sb strings.Builder
		descs := rows.FieldDescriptions()
		headers := make([]string, len(descs))
		for i, d := range descs {
			headers[i] = string(d.Name)
		}
		sb.WriteString(strings.Join(headers, " │ "))
		sb.WriteString("\n")
		sb.WriteString(strings.Repeat("─", 80))
		sb.WriteString("\n")

		count := 0
		for rows.Next() && count < 200 {
			vals, err := rows.Values()
			if err != nil {
				continue
			}
			parts := make([]string, len(vals))
			for i, v := range vals {
				parts[i] = fmt.Sprintf("%v", v)
			}
			sb.WriteString(strings.Join(parts, " │ "))
			sb.WriteString("\n")
			count++
		}
		if count == 200 {
			sb.WriteString("… (limited to 200 rows)\n")
		}
		return msgSQL{result: sb.String()}
	}
}

func cmdGiveItem(playerID int64, template string, qty, quality int64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		if playerID == 0 {
			return msgMutate{err: fmt.Errorf("player ID required")}
		}
		template = strings.TrimSpace(template)
		if template == "" {
			return msgMutate{err: fmt.Errorf("item template required")}
		}
		if qty <= 0 {
			return msgMutate{err: fmt.Errorf("quantity must be > 0")}
		}
		ctx := context.Background()

		// Prefer the backpack (inventory_type=0) — that's where resources live.
		// Fall back to the first available inventory if not found.
		var invID int64
		var maxSlots int
		var maxVolume float64
		err := globalDB.QueryRow(ctx, `
			SELECT id, COALESCE(max_item_count, -1), COALESCE(max_item_volume, -1)
			FROM dune.inventories
			WHERE actor_id = $1 AND inventory_type = 0
			LIMIT 1`, playerID).Scan(&invID, &maxSlots, &maxVolume)
		if err != nil {
			err = globalDB.QueryRow(ctx,
				`SELECT id, COALESCE(max_item_count, -1), COALESCE(max_item_volume, -1)
				 FROM dune.inventories WHERE actor_id = $1 LIMIT 1`, playerID).Scan(&invID, &maxSlots, &maxVolume)
			if err != nil {
				return msgMutate{err: fmt.Errorf("find inventory: %w", err)}
			}
		}

		hasSlotCap := maxSlots > 0
		hasVolumeCap := maxVolume > 0

		type stackSlot struct {
			id   int64
			size int64
		}
		var stacks []stackSlot
		usedSlots := 0
		usedVolume := 0.0
		maxPos := int64(-1)
		missingVolumes := map[string]struct{}{}

		rows, err := globalDB.Query(ctx, `
			SELECT id, template_id, stack_size, quality_level, volume_override, position_index
			FROM dune.items
			WHERE inventory_id = $1`, invID)
		if err != nil {
			return msgMutate{err: err}
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var tmpl string
			var stackSize int64
			var qLevel int64
			var vol pgtype.Float8
			var pos int64
			if err := rows.Scan(&id, &tmpl, &stackSize, &qLevel, &vol, &pos); err != nil {
				continue
			}
			usedSlots++
			if pos > maxPos {
				maxPos = pos
			}
			if quality == 0 && qLevel == 0 && tmpl == template {
				stacks = append(stacks, stackSlot{id: id, size: stackSize})
			}
			if hasVolumeCap {
				itemVol := 0.0
				if vol.Valid && vol.Float64 > 0 {
					itemVol = vol.Float64
				} else if itemData.Items != nil {
					if rule, ok := itemData.Items[tmpl]; ok && rule.Volume > 0 {
						itemVol = rule.Volume
					} else if itemData.DefaultVolume > 0 {
						itemVol = itemData.DefaultVolume
					}
				} else if itemData.DefaultVolume > 0 {
					itemVol = itemData.DefaultVolume
				}
				if itemVol <= 0 {
					missingVolumes[tmpl] = struct{}{}
					continue
				}
				usedVolume += itemVol * float64(stackSize)
			}
		}
		if rows.Err() != nil {
			return msgMutate{err: rows.Err()}
		}
		if hasVolumeCap && len(missingVolumes) > 0 {
			return msgMutate{err: fmt.Errorf(
				"missing volume data for %s; add to item-data.json",
				describeMissingTemplates(missingVolumes))}
		}

		stackMax, err := resolveStackMax(ctx, template, quality)
		if err != nil {
			return msgMutate{err: err}
		}
		if stackMax < 1 {
			stackMax = 1
		}

		if hasVolumeCap {
			perItemVol, err := resolveItemVolume(ctx, template)
			if err != nil {
				return msgMutate{err: err}
			}
			if perItemVol <= 0 {
				return msgMutate{err: fmt.Errorf("volume must be > 0 for %s", template)}
			}
			availableVol := maxVolume - usedVolume
			if availableVol < 0 {
				availableVol = 0
			}
			maxByVolume := int64(math.Floor(availableVol / perItemVol))
			if maxByVolume < qty {
				return msgMutate{err: fmt.Errorf(
					"over weight limit: room for %d more %s (%.2f/%.2f volume used)",
					maxByVolume, template, usedVolume, maxVolume)}
			}
		}

		sort.Slice(stacks, func(i, j int) bool {
			return stacks[i].size > stacks[j].size
		})

		remaining := qty
		type stackUpdate struct {
			id  int64
			add int64
		}
		var updates []stackUpdate
		if quality == 0 && stackMax > 1 {
			for _, st := range stacks {
				if remaining == 0 {
					break
				}
				space := stackMax - st.size
				if space <= 0 {
					continue
				}
				add := space
				if add > remaining {
					add = remaining
				}
				updates = append(updates, stackUpdate{id: st.id, add: add})
				remaining -= add
			}
		}

		var newStacks []int64
		for remaining > 0 {
			size := stackMax
			if size > remaining {
				size = remaining
			}
			newStacks = append(newStacks, size)
			remaining -= size
		}

		if hasSlotCap {
			freeSlots := maxSlots - usedSlots
			if freeSlots < len(newStacks) {
				return msgMutate{err: fmt.Errorf(
					"inventory full: need %d free slots, have %d",
					len(newStacks), freeSlots)}
			}
		}

		tx, err := globalDB.Begin(ctx)
		if err != nil {
			return msgMutate{err: err}
		}
		defer tx.Rollback(ctx)

		for _, u := range updates {
			_, err = tx.Exec(ctx, `
				UPDATE dune.items
				SET stack_size = stack_size + $1
				WHERE id = $2`, u.add, u.id)
			if err != nil {
				return msgMutate{err: err}
			}
		}

		nextPos := maxPos + 1
		for _, size := range newStacks {
			_, err = tx.Exec(ctx, `
				INSERT INTO dune.items (inventory_id, stack_size, position_index, template_id, quality_level, stats)
				VALUES ($1, $2, $3, $4, $5, '{}')`,
				invID, size, nextPos, template, quality)
			if err != nil {
				return msgMutate{err: err}
			}
			nextPos++
		}

		if err := tx.Commit(ctx); err != nil {
			return msgMutate{err: err}
		}

		msg := fmt.Sprintf("Added %d × %s to player %d", qty, template, playerID)
		if len(updates) > 0 || len(newStacks) > 0 {
			msg = fmt.Sprintf(
				"Added %d × %s to player %d (%d stack(s) topped up, %d new stack(s))",
				qty, template, playerID, len(updates), len(newStacks))
		}
		return msgMutate{ok: msg}
	}
}

func cmdGiveCurrency(playerID int64, amount int64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		res, err := globalDB.Exec(context.Background(), `
			UPDATE dune.player_virtual_currency_balances
			SET balance = balance + $1
			WHERE player_controller_id = $2 AND currency_id = 0`,
			amount, playerID)
		if err != nil {
			return msgMutate{err: err}
		}
		if res.RowsAffected() == 0 {
			_, err = globalDB.Exec(context.Background(), `
				INSERT INTO dune.player_virtual_currency_balances
				(player_controller_id, currency_id, balance)
				VALUES ($1, 0, $2)`, playerID, amount)
			if err != nil {
				return msgMutate{err: err}
			}
		}
		return msgMutate{ok: fmt.Sprintf("Added %d Solaris to player %d", amount, playerID)}
	}
}

func cmdGiveFactionRep(actorID int64, factionID int16, delta int32) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		ctx := context.Background()
		return applyFactionRepDelta(ctx, actorID, factionID, delta)
	}
}

func cmdGiveLandsraadScrip(actorID int64, delta int32) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		ctx := context.Background()
		if actorID == 0 {
			return msgMutate{err: fmt.Errorf("player ID required")}
		}
		currencyID, err := resolveScripCurrencyID(ctx)
		if err != nil {
			return msgMutate{err: err}
		}
		_, err = globalDB.Exec(ctx, `
			SELECT dune.adjust_player_virtual_currency_balance($1, $2, $3)`,
			actorID, currencyID, int64(delta))
		if err != nil {
			return msgMutate{err: err}
		}
		var balance int64
		_ = globalDB.QueryRow(ctx, `
			SELECT balance FROM dune.player_virtual_currency_balances
			WHERE player_controller_id = $1 AND currency_id = $2`,
			actorID, currencyID).Scan(&balance)
		return msgMutate{ok: fmt.Sprintf(
			"Added %d scrips (currency %d) to player %d — new balance %d",
			delta, currencyID, actorID, balance)}
	}
}

func cmdAwardXP(playerID int64, trackType string, delta int32) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		res, err := globalDB.Exec(context.Background(), `
			UPDATE dune.specialization_tracks
			SET xp_amount = xp_amount + $1
			WHERE player_id = $2 AND track_type::text = $3`,
			delta, playerID, trackType)
		if err != nil {
			return msgMutate{err: err}
		}
		if res.RowsAffected() == 0 {
			_, err = globalDB.Exec(context.Background(), `
				INSERT INTO dune.specialization_tracks (player_id, track_type, xp_amount, level)
				VALUES ($1, $2::dune.specializationtracktype, $3, 0)`, playerID, trackType, delta)
			if err != nil {
				return msgMutate{err: err}
			}
		}
		return msgMutate{ok: fmt.Sprintf("Awarded %d XP (%s) to player %d", delta, trackType, playerID)}
	}
}

// ── private helpers ───────────────────────────────────────────────────────────

func resolveStackMax(ctx context.Context, template string, quality int64) (int64, error) {
	if quality > 0 {
		return 1, nil
	}
	if itemData.Items != nil {
		if rule, ok := itemData.Items[template]; ok && rule.StackMax > 0 {
			return rule.StackMax, nil
		}
	}
	var maxStack int64
	err := globalDB.QueryRow(ctx, `
		SELECT COALESCE(MAX(stack_size), 0)
		FROM dune.items
		WHERE template_id = $1 AND quality_level = 0`, template).Scan(&maxStack)
	if err != nil {
		return 0, err
	}
	if maxStack > 0 {
		return maxStack, nil
	}
	if itemData.DefaultStackMax > 0 {
		return itemData.DefaultStackMax, nil
	}
	return 0, fmt.Errorf("stack max unknown for template %s", template)
}

func resolveItemVolume(ctx context.Context, template string) (float64, error) {
	if itemData.Items != nil {
		if rule, ok := itemData.Items[template]; ok && rule.Volume > 0 {
			return rule.Volume, nil
		}
	}
	var vol pgtype.Float8
	err := globalDB.QueryRow(ctx, `
		SELECT MAX(volume_override)
		FROM dune.items
		WHERE template_id = $1 AND volume_override IS NOT NULL`, template).Scan(&vol)
	if err != nil {
		return 0, err
	}
	if vol.Valid && vol.Float64 > 0 {
		return vol.Float64, nil
	}
	if itemData.DefaultVolume > 0 {
		return itemData.DefaultVolume, nil
	}
	return 0, fmt.Errorf("volume unknown for template %s", template)
}

func describeMissingTemplates(m map[string]struct{}) string {
	if len(m) == 0 {
		return ""
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	if len(names) > 5 {
		return strings.Join(names[:5], ", ") + ", …"
	}
	return strings.Join(names, ", ")
}

func formatCurrencyIDs(ids []int16) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	return strings.Join(parts, ", ")
}

func resolveScripCurrencyID(ctx context.Context) (int16, error) {
	if scripCurrencyID >= 0 {
		return int16(scripCurrencyID), nil
	}
	rows, err := globalDB.Query(ctx, `
		SELECT currency_id, COALESCE(SUM(balance), 0) AS total
		FROM dune.player_virtual_currency_balances
		WHERE currency_id <> get_solaris_id()
		GROUP BY currency_id
		ORDER BY total DESC, currency_id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var ids []int16
	for rows.Next() {
		var id int16
		var total int64
		if err := rows.Scan(&id, &total); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		return 0, rows.Err()
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("no non-solaris currency rows found; pass -scripcurrency")
	}
	return 0, fmt.Errorf("multiple non-solaris currency IDs found (%s); pass -scripcurrency", formatCurrencyIDs(ids))
}

func applyFactionRepDelta(ctx context.Context, actorID int64, factionID int16, delta int32) msgMutate {
	// Upsert reputation.
	res, err := globalDB.Exec(ctx, `
		UPDATE dune.player_faction_reputation
		SET reputation_amount = reputation_amount + $1
		WHERE actor_id = $2 AND faction_id = $3`,
		delta, actorID, factionID)
	if err != nil {
		return msgMutate{err: err}
	}
	if res.RowsAffected() == 0 {
		_, err = globalDB.Exec(ctx, `
			INSERT INTO dune.player_faction_reputation (actor_id, faction_id, reputation_amount)
			VALUES ($1, $2, $3)`, actorID, factionID, delta)
		if err != nil {
			return msgMutate{err: err}
		}
	}

	// Read the new total.
	var newTotal int32
	_ = globalDB.QueryRow(ctx, `
		SELECT reputation_amount FROM dune.player_faction_reputation
		WHERE actor_id = $1 AND faction_id = $2`, actorID, factionID).Scan(&newTotal)

	// Sync faction tier tags in player_tags.
	// Tags are keyed to account_id, looked up from the actor's owner.
	var accountID int64
	err = globalDB.QueryRow(ctx,
		`SELECT owner_account_id FROM dune.actors WHERE id = $1`, actorID).Scan(&accountID)
	if err == nil && accountID > 0 {
		fName := factionTagName(factionID)
		if fName != "" {
			syncFactionTierTags(ctx, accountID, fName, newTotal)
		}
	}

	fName := factionDisplayName(factionID)
	return msgMutate{ok: fmt.Sprintf(
		"Set %s scrips to %d (delta %+d) for actor %d — tier tags synced",
		fName, newTotal, delta, actorID)}
}

// factionTagName returns the game tag prefix for a faction (e.g. "Atreides").
func factionTagName(id int16) string {
	switch id {
	case 1:
		return "Atreides"
	case 2:
		return "Harkonnen"
	case 4:
		return "Smuggler"
	default:
		return ""
	}
}

func factionDisplayName(id int16) string {
	switch id {
	case 1:
		return "Atreides"
	case 2:
		return "Harkonnen"
	case 3:
		return "None"
	case 4:
		return "Smuggler"
	default:
		return fmt.Sprintf("Faction%d", id)
	}
}

// factionTierThresholds maps tier number to minimum reputation required.
// Observed from player_tags (Tier0–Tier5 all present at 11975 rep).
// Thresholds are estimates based on typical Dune Awakening faction rank design;
// the server re-evaluates on login, so over-granting is safe.
var factionTierThresholds = []int32{0, 2000, 5000, 9000, 14000, 20000}

// syncFactionTierTags inserts any tier tags the player should have based on
// their current reputation, and removes any they shouldn't have.
func syncFactionTierTags(ctx context.Context, accountID int64, faction string, rep int32) {
	if globalDB == nil {
		return
	}
	for tier, threshold := range factionTierThresholds {
		tag := fmt.Sprintf("Faction.%s.Tier%d", faction, tier)
		if rep >= threshold {
			// INSERT OR IGNORE equivalent
			globalDB.Exec(ctx, `
				INSERT INTO dune.player_tags (account_id, tag)
				VALUES ($1, $2)
				ON CONFLICT DO NOTHING`, accountID, tag)
		} else {
			globalDB.Exec(ctx, `
				DELETE FROM dune.player_tags WHERE account_id = $1 AND tag = $2`,
				accountID, tag)
		}
	}
}

// ── database tab types and fetch functions ────────────────────────────────────

type tableRow struct {
	Name     string
	RowCount int64
}

type columnInfo struct {
	Name     string
	DataType string
	Nullable string
}

type msgTables struct {
	rows []tableRow
	err  error
}

type msgDescribe struct {
	table string
	cols  []columnInfo
	err   error
}

type msgSample struct {
	table   string
	headers []string
	rows    [][]string
	err     error
}

type msgSearchCols struct {
	headers []string
	rows    [][]string
	err     error
}

func cmdFetchTables() tea.Msg {
	if globalDB == nil {
		return msgTables{err: fmt.Errorf("not connected")}
	}
	rows, err := globalDB.Query(context.Background(), `
		SELECT relname, COALESCE(n_live_tup, 0)
		FROM pg_stat_user_tables
		ORDER BY relname`)
	if err != nil {
		return msgTables{err: err}
	}
	defer rows.Close()
	var result []tableRow
	for rows.Next() {
		var r tableRow
		if err := rows.Scan(&r.Name, &r.RowCount); err != nil {
			return msgTables{err: err}
		}
		result = append(result, r)
	}
	return msgTables{rows: result}
}

func cmdDescribeTable(tbl string) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgDescribe{err: fmt.Errorf("not connected")}
		}
		rows, err := globalDB.Query(context.Background(), `
			SELECT column_name, data_type,
			       CASE is_nullable WHEN 'YES' THEN 'null' ELSE 'not null' END
			FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2
			ORDER BY ordinal_position`, dbSchema, tbl)
		if err != nil {
			return msgDescribe{table: tbl, err: err}
		}
		defer rows.Close()
		var cols []columnInfo
		for rows.Next() {
			var c columnInfo
			if err := rows.Scan(&c.Name, &c.DataType, &c.Nullable); err != nil {
				return msgDescribe{table: tbl, err: err}
			}
			cols = append(cols, c)
		}
		if err := rows.Err(); err != nil {
			return msgDescribe{table: tbl, err: err}
		}
		return msgDescribe{table: tbl, cols: cols}
	}
}

func cmdSampleTable(tbl string, limit int) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgSample{err: fmt.Errorf("not connected")}
		}
		// tbl comes from pg_stat_user_tables (server-side), not raw user input
		rows, err := globalDB.Query(context.Background(),
			fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d", dbSchema, tbl, limit))
		if err != nil {
			return msgSample{table: tbl, err: err}
		}
		defer rows.Close()
		var headers []string
		for _, fd := range rows.FieldDescriptions() {
			headers = append(headers, fd.Name)
		}
		var result [][]string
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				return msgSample{table: tbl, err: err}
			}
			var row []string
			for _, v := range vals {
				row = append(row, fmt.Sprintf("%v", v))
			}
			result = append(result, row)
		}
		if err := rows.Err(); err != nil {
			return msgSample{table: tbl, err: err}
		}
		return msgSample{table: tbl, headers: headers, rows: result}
	}
}

func cmdSearchColumns(term string) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgSearchCols{err: fmt.Errorf("not connected")}
		}
		rows, err := globalDB.Query(context.Background(), `
			SELECT table_name, column_name, data_type
			FROM information_schema.columns
			WHERE table_schema = $1
			  AND (column_name ILIKE $2 OR table_name ILIKE $2)
			ORDER BY table_name, column_name`, dbSchema, "%"+term+"%")
		if err != nil {
			return msgSearchCols{err: err}
		}
		defer rows.Close()
		headers := []string{"table", "column", "type"}
		var result [][]string
		for rows.Next() {
			var table, col, dtype string
			if err := rows.Scan(&table, &col, &dtype); err != nil {
				return msgSearchCols{err: err}
			}
			result = append(result, []string{table, col, dtype})
		}
		if err := rows.Err(); err != nil {
			return msgSearchCols{err: err}
		}
		return msgSearchCols{headers: headers, rows: result}
	}
}
