package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ── data fetch commands ───────────────────────────────────────────────────────

func cmdFetchPlayersBackground() tea.Cmd {
	return func() tea.Msg {
		msg := cmdFetchPlayers()
		if players, ok := msg.(msgPlayers); ok {
			return msgPlayersBackground(players)
		}
		return msg
	}
}

func cmdFetchOnlineStateBackground() tea.Cmd {
	return func() tea.Msg {
		msg := cmdFetchOnlineState()
		if online, ok := msg.(msgOnlineState); ok {
			return msgOnlineStateBackground(online)
		}
		return msg
	}
}

func cmdFetchPlayers() tea.Msg {
	if globalDB == nil {
		return msgPlayers{err: fmt.Errorf("not connected")}
	}
	rows, err := globalDB.Query(context.Background(), `
		SELECT a.id,
		       COALESCE(a.owner_account_id, 0),
		       COALESCE(ps.character_name, convert_from(e.encrypted_funcom_id, 'UTF8'), ''),
		       COALESCE(ps.player_controller_id, 0),
		       a.class,
		       COALESCE(a.map, ''),
		       COALESCE(pf.faction_id, 0),
		       COALESCE(ps.online_status::text, '')
		FROM dune.actors a
		LEFT JOIN dune.player_state ps ON ps.account_id = a.owner_account_id
		LEFT JOIN dune.encrypted_accounts e ON e.id = a.owner_account_id
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
		if err := rows.Scan(&p.ID, &p.AccountID, &p.Name, &p.ControllerID, &p.Class, &p.Map, &p.FactionID, &p.Status); err != nil {
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
			WHERE inv.actor_id = $1::bigint
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
			AND vcb.currency_id = $1::smallint
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
		sql = strings.TrimSpace(sql)
		if sql == "" {
			return msgSQL{err: fmt.Errorf("SQL required")}
		}

		rows, err := globalDB.Query(context.Background(), sql)
		if err != nil {
			return msgSQL{err: err}
		}

		var sb strings.Builder
		descs := rows.FieldDescriptions()
		if len(descs) > 0 {
			headers := make([]string, len(descs))
			for i, d := range descs {
				headers[i] = string(d.Name)
			}
			sb.WriteString(strings.Join(headers, " │ "))
			sb.WriteString("\n")
			sb.WriteString(strings.Repeat("─", 80))
			sb.WriteString("\n")
		}

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
		if err := rows.Err(); err != nil {
			rows.Close()
			return msgSQL{err: err}
		}
		rows.Close()

		tag := rows.CommandTag()
		if len(descs) == 0 {
			if tag.RowsAffected() >= 0 {
				return msgSQL{result: fmt.Sprintf("OK: %s (%d rows affected)", tag.String(), tag.RowsAffected())}
			}
			return msgSQL{result: "OK: " + tag.String()}
		}
		if count == 0 {
			sb.WriteString(fmt.Sprintf("(0 rows) — %s\n", tag.String()))
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
		template = resolveItemTemplateInput(strings.TrimSpace(template))
		if template == "" {
			return msgMutate{err: fmt.Errorf("item template required")}
		}
		if qty <= 0 {
			return msgMutate{err: fmt.Errorf("quantity must be > 0")}
		}
		ctx := context.Background()
		if isSolarisTemplate(template) {
			controllerID, err := resolveControllerIDForPawn(ctx, playerID)
			if err != nil {
				return msgMutate{err: err}
			}
			return applySolarisDelta(ctx, controllerID, qty)
		}

		// Prefer the backpack (inventory_type=0) — that's where resources live.
		// Fall back to the first available inventory if not found.
		var invID int64
		var maxSlots int
		var maxVolume float64
		err := globalDB.QueryRow(ctx, `
			SELECT id, COALESCE(max_item_count, -1), COALESCE(max_item_volume, -1)
			FROM dune.inventories
			WHERE actor_id = $1::bigint AND inventory_type = 0
			LIMIT 1`, playerID).Scan(&invID, &maxSlots, &maxVolume)
		if err != nil {
			err = globalDB.QueryRow(ctx,
				`SELECT id, COALESCE(max_item_count, -1), COALESCE(max_item_volume, -1)
				 FROM dune.inventories WHERE actor_id = $1::bigint LIMIT 1`, playerID).Scan(&invID, &maxSlots, &maxVolume)
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

		rows, err := globalDB.Query(ctx, `
			SELECT id, template_id, stack_size, quality_level, volume_override, position_index
			FROM dune.items
			WHERE inventory_id = $1::bigint`, invID)
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
			if quality == 0 && qLevel == 0 && strings.EqualFold(tmpl, template) {
				stacks = append(stacks, stackSlot{id: id, size: stackSize})
			}
			if hasVolumeCap {
				itemVol := 0.0
				if vol.Valid && vol.Float64 > 0 {
					itemVol = vol.Float64
				} else if itemData.Items != nil {
					if rule, ok := itemData.Items[strings.ToLower(tmpl)]; ok {
						itemVol = rule.Volume // 0 is valid — item takes no volume
					} else if itemData.DefaultVolume > 0 {
						itemVol = itemData.DefaultVolume
					} else {
						// Unknown templates are treated as zero-volume so admin grants
						// are not blocked by incomplete item-data.json coverage.
						itemVol = 0
					}
				} else if itemData.DefaultVolume > 0 {
					itemVol = itemData.DefaultVolume
				}
				usedVolume += itemVol * float64(stackSize)
			}
		}
		if rows.Err() != nil {
			return msgMutate{err: rows.Err()}
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
			if perItemVol > 0 {
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
			// perItemVol == 0: item takes no volume, always fits.
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
				SET stack_size = stack_size + $1::bigint
				WHERE id = $2::bigint`, u.add, u.id)
			if err != nil {
				return msgMutate{err: err}
			}
		}

		nextPos := maxPos + 1
		for _, size := range newStacks {
			_, err = tx.Exec(ctx, `
				INSERT INTO dune.items (inventory_id, stack_size, position_index, template_id, quality_level, stats)
				VALUES ($1::bigint, $2::bigint, $3::bigint, $4::text, $5::bigint, '{}'::jsonb)`,
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
		ctx := context.Background()
		return applySolarisDelta(ctx, playerID, amount)
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
		balance, err := applyCurrencyDelta(ctx, actorID, currencyID, int64(delta))
		if err != nil {
			return msgMutate{err: err}
		}
		return msgMutate{ok: fmt.Sprintf(
			"Added %d scrips (currency %d) to player %d — new balance %d",
			delta, currencyID, actorID, balance)}
	}
}

func cmdSetPlayerVitals(playerID int64, health, hydration float64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		if playerID == 0 {
			return msgMutate{err: fmt.Errorf("player ID required")}
		}
		if health < 0 {
			return msgMutate{err: fmt.Errorf("health must be >= 0")}
		}
		if hydration < 0 {
			return msgMutate{err: fmt.Errorf("hydration must be >= 0")}
		}

		ctx := context.Background()
		res, err := globalDB.Exec(ctx, `
			UPDATE dune.actors
			SET properties = jsonb_set(
				jsonb_set(
					properties,
					'{DamageableActorComponent,m_TotalMaxHealth}',
					to_jsonb($2::float8),
					false
				),
				'{DamageableActorComponent,m_CurrentMaxHealth}',
				to_jsonb($2::float8),
				false
			),
			gas_attributes = jsonb_set(
				jsonb_set(
					gas_attributes,
					'{DuneHydrationAttributeSet,CurrentHydration,CurrentValue}',
					to_jsonb($3::float8),
					false
				),
				'{DuneHydrationAttributeSet,CurrentHydration,BaseValue}',
				to_jsonb($3::float8),
				false
			)
			WHERE id = $1::bigint
			  AND class ILIKE '%PlayerCharacter%'`,
			playerID, health, hydration)
		if err != nil {
			return msgMutate{err: fmt.Errorf("set vitals: %w", err)}
		}
		if res.RowsAffected() == 0 {
			return msgMutate{err: fmt.Errorf("player %d not found", playerID)}
		}
		return msgMutate{ok: fmt.Sprintf(
			"Set player %d health to %.2f and hydration to %.2f",
			playerID, health, hydration)}
	}
}

func cmdSetTechPoints(playerID int64, points int64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		if playerID == 0 {
			return msgMutate{err: fmt.Errorf("player ID required")}
		}
		if points < 0 {
			return msgMutate{err: fmt.Errorf("tech points must be >= 0")}
		}

		ctx := context.Background()
		res, err := globalDB.Exec(ctx, `
			UPDATE dune.actors
			SET properties = jsonb_set(
				properties,
				'{TechKnowledgePlayerComponent,m_TechKnowledgePoints}',
				to_jsonb($2::bigint),
				false
			)
			WHERE id = $1::bigint
			  AND class ILIKE '%PlayerCharacter%'
			  AND properties ? 'TechKnowledgePlayerComponent'`,
			playerID, points)
		if err != nil {
			return msgMutate{err: fmt.Errorf("set tech points: %w", err)}
		}
		if res.RowsAffected() == 0 {
			return msgMutate{err: fmt.Errorf("player %d has no TechKnowledgePlayerComponent", playerID)}
		}
		return msgMutate{ok: fmt.Sprintf(
			"Set player %d tech points to %d",
			playerID, points)}
	}
}

func cmdSetSkillPoints(playerID int64, points int64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		if playerID == 0 {
			return msgMutate{err: fmt.Errorf("player ID required")}
		}
		if points < 0 {
			return msgMutate{err: fmt.Errorf("skill points must be >= 0")}
		}
		res, err := globalDB.Exec(context.Background(), `
			UPDATE dune.fgl_entities f
			SET components = jsonb_set(
				f.components,
				'{FLevelComponent,1,UnspentSkillPoints}',
				to_jsonb($2::bigint),
				false
			)
			FROM dune.actor_fgl_entities afe
			WHERE afe.entity_id = f.entity_id
			  AND afe.actor_id = $1::bigint
			  AND f.components ? 'FLevelComponent'`, playerID, points)
		if err != nil {
			return msgMutate{err: fmt.Errorf("set skill points: %w", err)}
		}
		if res.RowsAffected() == 0 {
			return msgMutate{err: fmt.Errorf("player %d has no FLevelComponent", playerID)}
		}
		return msgMutate{ok: fmt.Sprintf("Set player %d unspent skill points to %d", playerID, points)}
	}
}

func cmdAddPlayerXP(playerID int64, delta int64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		if playerID == 0 {
			return msgMutate{err: fmt.Errorf("player ID required")}
		}
		var total int64
		err := globalDB.QueryRow(context.Background(), `
			UPDATE dune.fgl_entities f
			SET components = jsonb_set(
				f.components,
				'{FLevelComponent,1,TotalXPEarned}',
				to_jsonb(GREATEST(COALESCE((f.components #>> '{FLevelComponent,1,TotalXPEarned}')::bigint, 0) + $2::bigint, 0)),
				false
			)
			FROM dune.actor_fgl_entities afe
			WHERE afe.entity_id = f.entity_id
			  AND afe.actor_id = $1::bigint
			  AND f.components ? 'FLevelComponent'
			RETURNING (f.components #>> '{FLevelComponent,1,TotalXPEarned}')::bigint`, playerID, delta).Scan(&total)
		if err != nil {
			if err == pgx.ErrNoRows {
				return msgMutate{err: fmt.Errorf("player %d has no FLevelComponent", playerID)}
			}
			return msgMutate{err: fmt.Errorf("add player XP: %w", err)}
		}
		return msgMutate{ok: fmt.Sprintf("Added %d player XP to player %d — total XP %d", delta, playerID, total)}
	}
}

func cmdSetProgression(playerID, totalXP, totalSkillPoints, unspentSkillPoints, techPoints int64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		if playerID == 0 {
			return msgMutate{err: fmt.Errorf("player ID required")}
		}
		if totalXP < 0 || totalSkillPoints < 0 || unspentSkillPoints < 0 || techPoints < 0 {
			return msgMutate{err: fmt.Errorf("progression values must be >= 0")}
		}
		ctx := context.Background()
		tx, err := globalDB.Begin(ctx)
		if err != nil {
			return msgMutate{err: err}
		}
		defer tx.Rollback(ctx)

		res, err := tx.Exec(ctx, `
			UPDATE dune.fgl_entities f
			SET components = jsonb_set(
				jsonb_set(
					jsonb_set(
						f.components,
						'{FLevelComponent,1,TotalXPEarned}',
						to_jsonb($2::bigint),
						false
					),
					'{FLevelComponent,1,TotalSkillPoints}',
					to_jsonb($3::bigint),
					false
				),
				'{FLevelComponent,1,UnspentSkillPoints}',
				to_jsonb($4::bigint),
				false
			)
			FROM dune.actor_fgl_entities afe
			WHERE afe.entity_id = f.entity_id
			  AND afe.actor_id = $1::bigint
			  AND f.components ? 'FLevelComponent'`,
			playerID, totalXP, totalSkillPoints, unspentSkillPoints)
		if err != nil {
			return msgMutate{err: fmt.Errorf("set player progression: %w", err)}
		}
		if res.RowsAffected() == 0 {
			return msgMutate{err: fmt.Errorf("player %d has no FLevelComponent", playerID)}
		}

		res, err = tx.Exec(ctx, `
			UPDATE dune.actors
			SET properties = jsonb_set(
				properties,
				'{TechKnowledgePlayerComponent,m_TechKnowledgePoints}',
				to_jsonb($2::bigint),
				false
			)
			WHERE id = $1::bigint
			  AND class ILIKE '%PlayerCharacter%'
			  AND properties ? 'TechKnowledgePlayerComponent'`,
			playerID, techPoints)
		if err != nil {
			return msgMutate{err: fmt.Errorf("set tech points: %w", err)}
		}
		if res.RowsAffected() == 0 {
			return msgMutate{err: fmt.Errorf("player %d has no TechKnowledgePlayerComponent", playerID)}
		}
		if err := tx.Commit(ctx); err != nil {
			return msgMutate{err: err}
		}
		return msgMutate{ok: fmt.Sprintf(
			"Set player %d progression: XP %d, skill %d total/%d unspent, tech %d",
			playerID, totalXP, totalSkillPoints, unspentSkillPoints, techPoints)}
	}
}

func cmdUnlockAllSkills(playerID int64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		if playerID == 0 {
			return msgMutate{err: fmt.Errorf("player ID required")}
		}
		res, err := globalDB.Exec(context.Background(), `
			UPDATE dune.fgl_entities f
			SET components = jsonb_set(
				f.components,
				'{FLevelComponent,1,ModuleData}',
				(
					SELECT jsonb_object_agg(
						key,
						jsonb_set(
							value,
							'{SkillPointsSpent}',
							to_jsonb(GREATEST(COALESCE((value->>'SkillPointsSpent')::integer, 0), 1)),
							true
						)
					)
					FROM jsonb_each(f.components #> '{FLevelComponent,1,ModuleData}')
				),
				false
			)
			FROM dune.actor_fgl_entities afe
			WHERE afe.entity_id = f.entity_id
			  AND afe.actor_id = $1::bigint
			  AND f.components ? 'FLevelComponent'
			  AND f.components #> '{FLevelComponent,1,ModuleData}' IS NOT NULL`, playerID)
		if err != nil {
			return msgMutate{err: fmt.Errorf("unlock all skills: %w", err)}
		}
		if res.RowsAffected() == 0 {
			return msgMutate{err: fmt.Errorf("player %d has no FLevelComponent ModuleData", playerID)}
		}
		return msgMutate{ok: fmt.Sprintf("Unlocked all FLevelComponent skill modules for player %d", playerID)}
	}
}

func cmdAwardXP(playerID int64, trackType string, delta int32) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		res, err := globalDB.Exec(context.Background(), `
			UPDATE dune.specialization_tracks
			SET xp_amount = xp_amount + $1::integer
			WHERE player_id = $2::bigint AND track_type::text = $3::text`,
			delta, playerID, trackType)
		if err != nil {
			return msgMutate{err: err}
		}
		if res.RowsAffected() == 0 {
			_, err = globalDB.Exec(context.Background(), `
				INSERT INTO dune.specialization_tracks (player_id, track_type, xp_amount, level)
				VALUES ($1::bigint, $2::dune.specializationtracktype, $3::integer, 0::real)`, playerID, trackType, delta)
			if err != nil {
				return msgMutate{err: err}
			}
		}
		return msgMutate{ok: fmt.Sprintf("Awarded %d XP (%s) to player %d", delta, trackType, playerID)}
	}
}

func cmdKickPlayer(playerID int64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		if playerID == 0 {
			return msgMutate{err: fmt.Errorf("player ID required")}
		}
		ctx := context.Background()

		// Set online_status to LoggingOut on the player_state row.
		// The server reads this on its next heartbeat and disconnects the session
		// without touching any game data (inventory, buildings, etc. are untouched).
		res, err := globalDB.Exec(ctx, `
			UPDATE dune.player_state
			SET online_status = 'LoggingOut'::dune.playerconnectionstatus
			WHERE player_pawn_id = $1::bigint
			   OR player_controller_id = $1::bigint`, playerID)
		if err != nil {
			return msgMutate{err: fmt.Errorf("kick: %w", err)}
		}
		if res.RowsAffected() == 0 {
			return msgMutate{err: fmt.Errorf("no player_state found for pawn/controller actor %d", playerID)}
		}
		return msgMutate{ok: fmt.Sprintf("Set pawn/controller actor %d → LoggingOut — server will disconnect on next heartbeat", playerID)}
	}
}

func cmdDeleteItem(itemID int64) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		if itemID == 0 {
			return msgMutate{err: fmt.Errorf("item ID required")}
		}
		ctx := context.Background()
		res, err := globalDB.Exec(ctx, `DELETE FROM dune.items WHERE id = $1::bigint`, itemID)
		if err != nil {
			return msgMutate{err: fmt.Errorf("delete item: %w", err)}
		}
		if res.RowsAffected() == 0 {
			return msgMutate{err: fmt.Errorf("item %d not found", itemID)}
		}
		return msgMutate{ok: fmt.Sprintf("Deleted item %d", itemID)}
	}
}

func cmdResetSpecializations(playerID int64, trackType string) tea.Cmd {
	return func() tea.Msg {
		if globalDB == nil {
			return msgMutate{err: fmt.Errorf("not connected")}
		}
		if playerID == 0 {
			return msgMutate{err: fmt.Errorf("player ID required")}
		}
		ctx := context.Background()

		var tracksDeleted, keystonesDeleted int64
		if trackType == "" || strings.EqualFold(trackType, "all") {
			res, err := globalDB.Exec(ctx,
				`DELETE FROM dune.specialization_tracks WHERE player_id = $1::bigint`, playerID)
			if err != nil {
				return msgMutate{err: fmt.Errorf("reset tracks: %w", err)}
			}
			tracksDeleted = res.RowsAffected()

			res, err = globalDB.Exec(ctx,
				`DELETE FROM dune.purchased_specialization_keystones WHERE player_id = $1::bigint`, playerID)
			if err != nil {
				return msgMutate{err: fmt.Errorf("reset keystones: %w", err)}
			}
			keystonesDeleted = res.RowsAffected()
		} else {
			res, err := globalDB.Exec(ctx, `
				DELETE FROM dune.specialization_tracks
				WHERE player_id = $1::bigint AND track_type::text = $2::text`, playerID, trackType)
			if err != nil {
				return msgMutate{err: fmt.Errorf("reset track: %w", err)}
			}
			tracksDeleted = res.RowsAffected()
		}

		if trackType == "" || strings.EqualFold(trackType, "all") {
			return msgMutate{ok: fmt.Sprintf(
				"Reset player %d: %d track(s) + %d keystone(s) cleared",
				playerID, tracksDeleted, keystonesDeleted)}
		}
		return msgMutate{ok: fmt.Sprintf(
			"Reset %s track for player %d (%d row(s) cleared)", trackType, playerID, tracksDeleted)}
	}
}

// onlineStateRow holds a single row from the player online state query.
type onlineStateRow struct {
	PlayerID int64
	Name     string
	Map      string
	Status   string
	LastSeen string
}

type msgOnlineState struct {
	rows []onlineStateRow
	err  error
}

func cmdFetchOnlineState() tea.Msg {
	if globalDB == nil {
		return msgOnlineState{err: fmt.Errorf("not connected")}
	}
	rows, err := globalDB.Query(context.Background(), `
		SELECT ps.player_controller_id,
		       COALESCE(ps.character_name, ''),
		       COALESCE(a.map, ''),
		       ps.online_status::text,
		       COALESCE(to_char(ps.last_avatar_activity AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS'), '')
		FROM dune.player_state ps
		LEFT JOIN dune.actors a ON a.id = ps.player_controller_id
		ORDER BY ps.online_status DESC, ps.last_avatar_activity DESC`)
	if err != nil {
		return msgOnlineState{err: err}
	}
	defer rows.Close()

	var out []onlineStateRow
	for rows.Next() {
		var r onlineStateRow
		if err := rows.Scan(&r.PlayerID, &r.Name, &r.Map, &r.Status, &r.LastSeen); err != nil {
			continue
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return msgOnlineState{err: err}
	}
	return msgOnlineState{rows: out}
}

// structureCount holds building + totem counts for a player.
type structureCount struct {
	PlayerAccountID int64
	Buildings       int64
	Totems          int64
}

type msgStructures struct {
	counts map[int64]structureCount
	err    error
}

func cmdFetchStructureCounts() tea.Msg {
	if globalDB == nil {
		return msgStructures{err: fmt.Errorf("not connected")}
	}
	rows, err := globalDB.Query(context.Background(), `
		SELECT a.owner_account_id,
		       SUM(CASE WHEN b.id IS NOT NULL THEN 1 ELSE 0 END) AS buildings,
		       SUM(CASE WHEN t.id IS NOT NULL THEN 1 ELSE 0 END) AS totems
		FROM dune.actors a
		LEFT JOIN dune.buildings b ON b.id = a.id
		LEFT JOIN dune.totems t ON t.id = a.id
		WHERE a.owner_account_id IS NOT NULL
		  AND (b.id IS NOT NULL OR t.id IS NOT NULL)
		GROUP BY a.owner_account_id`)
	if err != nil {
		return msgStructures{err: err}
	}
	defer rows.Close()

	counts := make(map[int64]structureCount)
	for rows.Next() {
		var accountID, bld, tot int64
		if err := rows.Scan(&accountID, &bld, &tot); err != nil {
			continue
		}
		counts[accountID] = structureCount{accountID, bld, tot}
	}
	if err := rows.Err(); err != nil {
		return msgStructures{err: err}
	}
	return msgStructures{counts: counts}
}

// ── private helpers ───────────────────────────────────────────────────────────

func resolveItemTemplateInput(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	for _, t := range dbItemTemplates {
		if strings.EqualFold(t, input) {
			return t
		}
	}
	if entry, ok := duneItemNames[strings.ToLower(input)]; ok && entry.ID != "" {
		return entry.ID
	}
	if itemData.Items != nil {
		if rule, ok := itemData.Items[strings.ToLower(input)]; ok {
			if rule.TemplateID != "" {
				return rule.TemplateID
			}
		}
	}
	for _, entry := range duneItemNames {
		if entry.Name != "" && strings.EqualFold(entry.Name, input) && entry.ID != "" {
			return entry.ID
		}
	}
	if itemData.Items != nil {
		for k, rule := range itemData.Items {
			if rule.Name != "" && strings.EqualFold(rule.Name, input) {
				if rule.TemplateID != "" {
					return rule.TemplateID
				}
				return k
			}
		}
	}
	return input
}

func isSolarisTemplate(template string) bool {
	switch strings.ToLower(strings.TrimSpace(template)) {
	case "solaris", "solariscoin", "solari", "solaris_coin":
		return true
	default:
		return false
	}
}

func resolveControllerIDForPawn(ctx context.Context, playerID int64) (int64, error) {
	if playerID == 0 {
		return 0, fmt.Errorf("player ID required")
	}
	var controllerID int64
	err := globalDB.QueryRow(ctx, `
		SELECT COALESCE(ps.player_controller_id, 0)
		FROM dune.actors a
		JOIN dune.player_state ps ON ps.account_id = a.owner_account_id
		WHERE a.id = $1::bigint`, playerID).Scan(&controllerID)
	if err != nil {
		return 0, fmt.Errorf("find player controller: %w", err)
	}
	if controllerID == 0 {
		return 0, fmt.Errorf("player %d has no controller ID", playerID)
	}
	return controllerID, nil
}

func applySolarisDelta(ctx context.Context, playerID int64, amount int64) tea.Msg {
	if playerID == 0 {
		return msgMutate{err: fmt.Errorf("player ID required")}
	}
	var currencyID int16
	if err := globalDB.QueryRow(ctx, `SELECT dune.get_solaris_id()`).Scan(&currencyID); err != nil {
		return msgMutate{err: fmt.Errorf("resolve Solaris currency id: %w", err)}
	}
	balance, err := applyCurrencyDelta(ctx, playerID, currencyID, amount)
	if err != nil {
		return msgMutate{err: err}
	}
	return msgMutate{ok: fmt.Sprintf(
		"Added %d Solaris to player %d — new balance %d",
		amount, playerID, balance)}
}

func applyCurrencyDelta(ctx context.Context, playerID int64, currencyID int16, amount int64) (int64, error) {
	if playerID == 0 {
		return 0, fmt.Errorf("player ID required")
	}
	tx, err := globalDB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var balance int64
	err = tx.QueryRow(ctx, `
		UPDATE dune.player_virtual_currency_balances
		SET balance = balance + $3::bigint
		WHERE player_controller_id = $1::bigint
		  AND currency_id = $2::smallint
		  AND balance + $3::bigint >= 0
		RETURNING balance`, playerID, currencyID, amount).Scan(&balance)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return 0, err
		}
		return balance, nil
	}
	if err != pgx.ErrNoRows {
		return 0, err
	}
	if amount < 0 {
		return 0, fmt.Errorf("insufficient currency balance")
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO dune.player_virtual_currency_balances (player_controller_id, currency_id, balance)
		VALUES ($1::bigint, $2::smallint, $3::bigint)
		RETURNING balance`, playerID, currencyID, amount).Scan(&balance)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return balance, nil
}

func resolveStackMax(ctx context.Context, template string, quality int64) (int64, error) {
	if quality > 0 {
		return 1, nil
	}
	if itemData.Items != nil {
		if rule, ok := itemData.Items[strings.ToLower(template)]; ok && rule.StackMax > 0 {
			return rule.StackMax, nil
		}
	}
	var maxStack int64
	err := globalDB.QueryRow(ctx, `
		SELECT COALESCE(MAX(stack_size), 0)
		FROM dune.items
		WHERE template_id = $1::text AND quality_level = 0`, template).Scan(&maxStack)
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
		if rule, ok := itemData.Items[strings.ToLower(template)]; ok {
			// volume=0 is valid (item takes no inventory space).
			return rule.Volume, nil
		}
	}
	var vol pgtype.Float8
	err := globalDB.QueryRow(ctx, `
		SELECT MAX(volume_override)
		FROM dune.items
		WHERE template_id = $1::text AND volume_override IS NOT NULL`, template).Scan(&vol)
	if err != nil {
		return 0, err
	}
	if vol.Valid && vol.Float64 > 0 {
		return vol.Float64, nil
	}
	if itemData.DefaultVolume > 0 {
		return itemData.DefaultVolume, nil
	}
	// Unknown templates are treated as zero-volume so item grants still work
	// when item-data.json has no entry and the DB has no prior volume sample.
	return 0, nil
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
		WHERE currency_id <> dune.get_solaris_id()
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
	// Route through set_player_faction_reputation which handles tier tags correctly.
	// First get current rep to compute the new absolute value.
	var currentRep int32
	_ = globalDB.QueryRow(ctx, `
		SELECT COALESCE(reputation_amount, 0) FROM dune.player_faction_reputation
		WHERE actor_id = $1::bigint AND faction_id = $2::smallint`, actorID, factionID).Scan(&currentRep)

	newRep := currentRep + delta
	if newRep < 0 {
		newRep = 0
	}

	// set_player_faction_reputation(actor_id, faction_id, new_reputation) handles
	// both the reputation update and tier tag synchronization server-side.
	_, err := globalDB.Exec(ctx, `
		SELECT dune.set_player_faction_reputation($1::bigint, $2::smallint, $3::integer)`,
		actorID, factionID, newRep)
	if err != nil {
		return msgMutate{err: fmt.Errorf("set_player_faction_reputation: %w", err)}
	}

	fName := factionDisplayName(factionID)
	return msgMutate{ok: fmt.Sprintf(
		"Set %s rep to %d (was %d, delta %+d) for actor %d — tier tags synced by server",
		fName, newRep, currentRep, delta, actorID)}
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

func cmdFetchItemTemplates() tea.Msg {
	if globalDB == nil {
		return msgItemTemplates{}
	}
	rows, err := globalDB.Query(context.Background(),
		`SELECT DISTINCT template_id FROM dune.items ORDER BY template_id`)
	if err != nil {
		return msgItemTemplates{}
	}
	defer rows.Close()
	var templates []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			templates = append(templates, t)
		}
	}
	return msgItemTemplates{templates: templates}
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
			WHERE table_schema = $1::text AND table_name = $2::text
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
		// Sanitize table name defensively even though tbl comes from pg_stat_user_tables.
		// pgx.Identifier handles quoting and escaping to prevent SQL injection.
		safeTable := pgx.Identifier{dbSchema, tbl}.Sanitize()
		rows, err := globalDB.Query(context.Background(),
			fmt.Sprintf("SELECT * FROM %s LIMIT %d", safeTable, limit))
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
			WHERE table_schema = $1::text
			  AND (column_name ILIKE $2::text OR table_name ILIKE $2::text)
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
