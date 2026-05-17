package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"
)

const (
	listingsPerItem = 5
	orderExpirySecs = int64(24 * 3600)
)

type categoryEntry struct {
	mask  int32
	depth int16
}

type listingInfo struct {
	orderID   int64
	itemID    int64
	stackSize int64
	price     int64
	grade     int64
}

type Exchange struct {
	db            *pgxpool.Pool
	cache         *sql.DB // local SQLite category cache
	segIdx        [4][]string
	botInvID      int64
	ownerID       int64 // actor ID of the market bot (Revy)
	exchangeID    int64
	accessPointID int64
	prices        map[string]int64
	categories    map[string]categoryEntry
	gameEpochUnix int64 // unix timestamp of the game server's time epoch; 0 = unknown
	nextPos       int64 // position_index counter for item inserts
	buyThreshold  float64
	maxBuys       int
}

func NewExchange(db *pgxpool.Pool, cachePath string, catalog []CatalogItem) (*Exchange, error) {
	cache, err := sql.Open("sqlite", cachePath)
	if err != nil {
		return nil, fmt.Errorf("open category cache %s: %w", cachePath, err)
	}
	if _, err := cache.Exec(`
		CREATE TABLE IF NOT EXISTS categories (
			template_id    TEXT     PRIMARY KEY,
			category_mask  INTEGER  NOT NULL,
			category_depth INTEGER  NOT NULL
		)`); err != nil {
		return nil, fmt.Errorf("init category cache: %w", err)
	}
	if _, err := cache.Exec(`
		CREATE TABLE IF NOT EXISTS metadata (
			key   TEXT    PRIMARY KEY,
			value INTEGER NOT NULL
		)`); err != nil {
		return nil, fmt.Errorf("init metadata cache: %w", err)
	}

	ex := &Exchange{
		db:         db,
		cache:      cache,
		segIdx:     buildSegmentIndex(catalog),
		prices:     make(map[string]int64),
		categories: make(map[string]categoryEntry),
	}
	cache.QueryRow(`SELECT value FROM metadata WHERE key = 'game_epoch_unix'`).Scan(&ex.gameEpochUnix)
	if ex.gameEpochUnix != 0 {
		log.Printf("loaded game epoch from cache: unix %d (game time now: %d)", ex.gameEpochUnix, ex.gameNow())
	}
	return ex, nil
}

func (e *Exchange) learnGameEpoch(ctx context.Context) {
	var ref int64
	err := e.db.QueryRow(ctx, `
		SELECT expiration_time FROM dune.dune_exchange_orders
		WHERE is_npc_order = FALSE
		  AND expiration_time IS NOT NULL
		  AND expiration_time < 1000000000
		ORDER BY expiration_time DESC LIMIT 1`).Scan(&ref)
	if err != nil || ref == 0 {
		return
	}
	gameNow := ref - int64(24*3600)
	epoch := time.Now().Unix() - gameNow
	if e.gameEpochUnix == epoch {
		return
	}
	e.gameEpochUnix = epoch
	e.cache.Exec(`INSERT INTO metadata (key, value) VALUES ('game_epoch_unix', ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, epoch)
	log.Printf("game epoch learned: unix %d (current game time: %d)", epoch, gameNow)
}

func (e *Exchange) gameNow() int64 {
	if e.gameEpochUnix == 0 {
		return 0
	}
	return time.Now().Unix() - e.gameEpochUnix
}

func (e *Exchange) Init(ctx context.Context, catalog []CatalogItem) error {
	err := e.db.QueryRow(ctx,
		`SELECT exchange_id FROM dune.dune_exchange_orders WHERE is_npc_order = FALSE LIMIT 1`).Scan(&e.exchangeID)
	if err != nil {
		// No player orders yet — fall back to first non-Global exchange
		if err2 := e.db.QueryRow(ctx,
			`SELECT id FROM dune.dune_exchanges WHERE exchange_name != 'Global' ORDER BY id LIMIT 1`).Scan(&e.exchangeID); err2 != nil {
			return fmt.Errorf("detect exchange id: %w", err2)
		}
	}
	log.Printf("exchange id: %d", e.exchangeID)

	if err := e.db.QueryRow(ctx,
		`SELECT DISTINCT access_point_id FROM dune.dune_exchange_orders WHERE exchange_id = $1 LIMIT 1`,
		e.exchangeID).Scan(&e.accessPointID); err != nil {
		e.accessPointID = 1
		log.Printf("access point: no existing orders, defaulting to 1")
	} else {
		log.Printf("access point id: %d", e.accessPointID)
	}

	if err := e.db.QueryRow(ctx,
		`SELECT dune.get_exchange_inventory_id($1)`, e.exchangeID).Scan(&e.botInvID); err != nil {
		return fmt.Errorf("exchange inventory: %w", err)
	}
	log.Printf("exchange inventory id: %d", e.botInvID)

	if err := e.initBotUser(ctx); err != nil {
		return fmt.Errorf("bot user: %w", err)
	}

	// Seed list prices.
	for _, item := range catalog {
		e.prices[item.TemplateID] = item.ListPrice
	}

	// Start position counter after existing items.
	e.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(position_index), -1) + 1 FROM dune.items WHERE inventory_id = $1`,
		e.botInvID).Scan(&e.nextPos)

	e.learnGameEpoch(ctx)
	e.refreshCategoryCache(ctx)

	if err := e.poisonCategoryHash(ctx); err != nil {
		log.Printf("warn: category hash poison: %v", err)
	}
	return nil
}

func (e *Exchange) initBotUser(ctx context.Context) error {
	err := e.db.QueryRow(ctx,
		`SELECT id FROM dune.actors WHERE class = 'Revy' LIMIT 1`).Scan(&e.ownerID)
	if err == pgx.ErrNoRows {
		err = e.db.QueryRow(ctx,
			`INSERT INTO dune.actors (class, serial, gas_attributes, properties, dimension_index)
			 VALUES ('Revy', 0, '{}', '{}', 0) RETURNING id`).Scan(&e.ownerID)
	}
	if err != nil {
		return fmt.Errorf("bot actor: %w", err)
	}
	log.Printf("bot actor id: %d (Revy)", e.ownerID)

	var userID int64
	if err := e.db.QueryRow(ctx,
		`SELECT dune.dune_exchange_get_user_id($1)`, e.ownerID).Scan(&userID); err != nil {
		return err
	}
	_, err = e.db.Exec(ctx,
		`SELECT dune.dune_exchange_modify_user_solari_balance($1, $2)`,
		e.ownerID, int64(9_000_000_000_000))
	return err
}

func (e *Exchange) poisonCategoryHash(ctx context.Context) error {
	_, err := e.db.Exec(ctx,
		`INSERT INTO dune.dune_exchange_categories_hash (id, hash) VALUES (1, 0)
		 ON CONFLICT (id) DO UPDATE SET hash = 0`)
	return err
}

func (e *Exchange) refreshCategoryCache(ctx context.Context) {
	rows, err := e.cache.Query(
		`SELECT template_id, category_mask, category_depth FROM categories`)
	if err != nil {
		log.Printf("warn: load category cache: %v", err)
	} else {
		for rows.Next() {
			var tmpl string
			var mask int32
			var depth int16
			if err := rows.Scan(&tmpl, &mask, &depth); err != nil {
				continue
			}
			e.categories[strings.ToLower(tmpl)] = categoryEntry{mask: mask, depth: depth}
		}
		rows.Close()
	}

	liveRows, err := e.db.Query(ctx, `
		SELECT DISTINCT template_id, category_mask, category_depth
		FROM dune.dune_exchange_orders
		WHERE category_mask != 0`)
	if err != nil {
		log.Printf("warn: live category scan: %v", err)
		return
	}
	defer liveRows.Close()

	type entry struct {
		tmpl  string
		mask  int32
		depth int16
	}
	var toWrite []entry
	for liveRows.Next() {
		var tmpl string
		var mask int32
		var depth int16
		if err := liveRows.Scan(&tmpl, &mask, &depth); err != nil {
			continue
		}
		key := strings.ToLower(tmpl)
		if _, known := e.categories[key]; !known {
			e.categories[key] = categoryEntry{mask: mask, depth: depth}
			toWrite = append(toWrite, entry{tmpl, mask, depth})
		}
	}

	for _, en := range toWrite {
		if _, err := e.cache.Exec(`
			INSERT INTO categories (template_id, category_mask, category_depth)
			VALUES (?, ?, ?)
			ON CONFLICT (template_id) DO UPDATE
			  SET category_mask  = excluded.category_mask,
			      category_depth = excluded.category_depth`,
			en.tmpl, en.mask, en.depth); err != nil {
			log.Printf("warn: persist category %s: %v", en.tmpl, err)
		}
	}

	if len(toWrite) > 0 {
		log.Printf("category cache: +%d new (total %d)", len(toWrite), len(e.categories))
	}
}

func (e *Exchange) categoryFor(item CatalogItem) (mask int32, depth int16) {
	if item.IsSchematic && item.Category != "" {
		if m, d, ok := UniqueSchematicsMask(item.Category); ok {
			return m, d
		}
		// Schematic whose category has no unique-schematics section — fall through.
		return CategoryMask(item.Category, e.segIdx)
	}
	// Catalog items always use a freshly computed mask so stale cache values
	// can never poison category filters.
	if item.Category != "" {
		return CategoryMask(item.Category, e.segIdx)
	}
	if c, ok := e.categories[strings.ToLower(item.TemplateID)]; ok {
		return c.mask, c.depth
	}
	return 0, 0
}

func (e *Exchange) buyPlayerListings(ctx context.Context, orderExpiry int64) {
	if e.buyThreshold <= 0 {
		return
	}
	if orderExpiry <= 0 {
		orderExpiry = 999_999_999
	}

	// Use actual current stack_size from items so partial fills pay the right amount.
	rows, err := e.db.Query(ctx, `
		SELECT o.id, o.template_id, o.item_price, o.item_id, o.owner_id,
		       COALESCE(i.stack_size, s.initial_stack_size) AS actual_stack
		FROM dune.dune_exchange_orders o
		JOIN dune.dune_exchange_sell_orders s ON s.order_id = o.id
		LEFT JOIN dune.items i ON i.id = o.item_id
		WHERE o.is_npc_order = FALSE AND o.exchange_id = $1
		LIMIT $2`, e.exchangeID, e.maxBuys*10)
	if err != nil {
		log.Printf("buy: query: %v", err)
		return
	}
	defer rows.Close()

	purchased, skippedPrice, skippedUnknown, errs := 0, 0, 0, 0

	for rows.Next() {
		if purchased >= e.maxBuys {
			break
		}

		var orderID, price, itemID, sellerActorID, stackSize int64
		var tmpl string
		if err := rows.Scan(&orderID, &tmpl, &price, &itemID, &sellerActorID, &stackSize); err != nil {
			errs++
			continue
		}

		botPrice, known := e.prices[tmpl]
		if !known || botPrice <= 0 {
			skippedUnknown++
			continue
		}
		if price > int64(float64(botPrice)*e.buyThreshold) {
			skippedPrice++
			continue
		}

		totalCost := price * stackSize

		tx, err := e.db.Begin(ctx)
		if err != nil {
			errs++
			continue
		}

		// Create a payment log entry for the seller (item_id omitted → NULL).
		// completion_type=4 + item_id=NULL is what the game engine uses for the
		// seller side of a fulfilled sale, causing the client to show "Take Solari"
		// in the Completed tab and fire the "X SOLARIS CLAIMED" toast on collection.
		var logOrderID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO dune.dune_exchange_orders
			  (exchange_id, access_point_id, owner_id, template_id, expiration_time,
			   durability_cur, durability_max, item_price, category_mask, category_depth, is_npc_order)
			VALUES ($1,$2,$3,$4,$5,1.0,1.0,$6,0,0,FALSE) RETURNING id`,
			e.exchangeID, e.accessPointID, sellerActorID, tmpl, orderExpiry, price,
		).Scan(&logOrderID); err != nil {
			log.Printf("buy: log order for %s: %v", tmpl, err)
			tx.Rollback(ctx)
			errs++
			continue
		}

		ok := true
		for _, q := range []struct {
			sql  string
			args []any
		}{
			// Fulfilled-order record: source_order_id=NULL (original listing will be
			// deleted), original_order_id kept for telemetry linkage (no FK constraint).
			{`INSERT INTO dune.dune_exchange_fulfilled_orders
			    (order_id, source_order_id, completion_type, stack_size, original_order_id)
			    VALUES ($1, NULL, 4, $2, $3)`, []any{logOrderID, stackSize, orderID}},
			// Debit bot's exchange balance for the purchase.
			{`UPDATE dune.dune_exchange_users
			    SET solari_balance = solari_balance - $1
			    WHERE owner_id = $2`, []any{totalCost, e.ownerID}},
			// Remove the original sell listing.
			{`DELETE FROM dune.dune_exchange_sell_orders WHERE order_id = $1`, []any{orderID}},
			{`DELETE FROM dune.dune_exchange_orders WHERE id = $1`, []any{orderID}},
		} {
			if _, err := tx.Exec(ctx, q.sql, q.args...); err != nil {
				log.Printf("buy: %s: %v", tmpl, err)
				ok = false
				break
			}
		}
		if ok && itemID > 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM dune.items WHERE id = $1`, itemID); err != nil {
				log.Printf("buy: delete item %d: %v", itemID, err)
				ok = false
			}
		}
		if !ok {
			tx.Rollback(ctx)
			errs++
			continue
		}
		if err := tx.Commit(ctx); err != nil {
			errs++
			continue
		}
		purchased++
	}

	if purchased+errs > 0 || skippedPrice > 0 {
		log.Printf("buy: %d purchased, %d skipped-price, %d skipped-unknown, %d errors",
			purchased, skippedPrice, skippedUnknown, errs)
	}
}

// gradeWeights maps rarity (lowercase) to per-grade weights for grades 1–5.
// Unique/memento items lean toward higher grades; common items lean lower.
var gradeWeights = map[string][]int{
	"unique":  {5, 10, 20, 30, 35},
	"memento": {5, 10, 20, 30, 35},
	"":        {35, 30, 20, 10, 5}, // common / unset
}

// gradeForItem picks a random quality_level (1–5) weighted by rarity.
// Stackable materials and schematics return 0 (no grade).
func gradeForItem(item CatalogItem) int64 {
	if item.StackMax > 1 || item.IsSchematic {
		return 0
	}
	weights, ok := gradeWeights[strings.ToLower(item.Rarity)]
	if !ok {
		weights = gradeWeights[""]
	}
	total := 0
	for _, w := range weights {
		total += w
	}
	r := rand.Intn(total)
	for i, w := range weights {
		r -= w
		if r < 0 {
			return int64(i + 1)
		}
	}
	return 5
}

// createListing inserts one sell order + its item directly into the DB.
func (e *Exchange) createListing(ctx context.Context, item CatalogItem, price, stackMax, expiry int64) error {
	catMask, catDepth := e.categoryFor(item)

	tx, err := e.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qualityLevel := gradeForItem(item)
	listPrice := gradedPrice(price, qualityLevel) // grade-adjusted; grade 0 returns base price unchanged

	// Item goes directly into the exchange inventory.
	var itemID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO dune.items (inventory_id, stack_size, position_index, template_id, quality_level, stats)
		VALUES ($1, $2, $3, $4, $5, '{}') RETURNING id`,
		e.botInvID, stackMax, e.nextPos, item.TemplateID, qualityLevel).Scan(&itemID); err != nil {
		return fmt.Errorf("insert item: %w", err)
	}
	e.nextPos++

	var orderID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO dune.dune_exchange_orders
		  (exchange_id, access_point_id, owner_id, is_npc_order, expiration_time,
		   template_id, durability_cur, durability_max, category_mask, category_depth,
		   item_price, quality_level, item_id)
		VALUES ($1,$2,$3,TRUE,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		e.exchangeID, e.accessPointID, e.ownerID, expiry,
		item.TemplateID, float32(1.0), float32(1.0),
		catMask, catDepth, listPrice, qualityLevel, itemID).Scan(&orderID); err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO dune.dune_exchange_sell_orders (order_id, initial_stack_size, wear_normalized_price)
		VALUES ($1, $2, $3)`,
		orderID, stackMax, listPrice); err != nil {
		return fmt.Errorf("insert sell order: %w", err)
	}

	return tx.Commit(ctx)
}

func (e *Exchange) Tick(ctx context.Context, catalog []CatalogItem) {
	e.learnGameEpoch(ctx)
	e.refreshCategoryCache(ctx)
	e.updatePrices(ctx, catalog)

	gameNow := e.gameNow()
	var orderExpiry int64
	if gameNow > 0 {
		orderExpiry = gameNow + orderExpirySecs
	} else {
		orderExpiry = 999_999_999
	}

	e.buyPlayerListings(ctx, orderExpiry)

	// Load all current bot listings.
	rows, err := e.db.Query(ctx, `
		SELECT o.id, o.template_id, o.item_id, o.item_price, i.stack_size, o.quality_level
		FROM dune.dune_exchange_orders o
		JOIN dune.items i ON i.id = o.item_id
		WHERE o.owner_id = $1 AND o.is_npc_order = TRUE`, e.ownerID)
	if err != nil {
		log.Printf("load listings: %v", err)
		return
	}
	current := make(map[string][]listingInfo)
	for rows.Next() {
		var orderID, itemID, price, stack, grade int64
		var tmpl string
		if err := rows.Scan(&orderID, &tmpl, &itemID, &price, &stack, &grade); err != nil {
			continue
		}
		current[tmpl] = append(current[tmpl], listingInfo{orderID, itemID, stack, price, grade})
	}
	rows.Close()

	created, topped, pruned, errs := 0, 0, 0, 0

	for _, item := range catalog {
		stackMax := item.StackMax
		if stackMax <= 0 {
			stackMax = 1
		}
		price := e.prices[item.TemplateID]
		if price <= 0 {
			price = item.ListPrice
		}
		if price <= 0 {
			continue // item has no meaningful price, skip
		}

		listings := current[item.TemplateID]

		// Remove listings at stale prices. Each listing's expected price is the
		// base price scaled by its grade, so a base-price change reprices all grades.
		var valid []listingInfo
		for _, l := range listings {
			if l.price != gradedPrice(price, l.grade) {
				e.db.Exec(ctx, `DELETE FROM dune.dune_exchange_orders WHERE id = $1`, l.orderID)
				e.db.Exec(ctx, `DELETE FROM dune.items WHERE id = $1`, l.itemID)
				pruned++
			} else {
				valid = append(valid, l)
			}
		}

		// Top up depleted listings and refresh expiry.
		for _, l := range valid {
			if l.stackSize < stackMax {
				e.db.Exec(ctx, `UPDATE dune.items SET stack_size = $1 WHERE id = $2`, stackMax, l.itemID)
				topped++
			}
			e.db.Exec(ctx,
				`UPDATE dune.dune_exchange_orders SET expiration_time = $1 WHERE id = $2`,
				orderExpiry, l.orderID)
		}

		// Create new listings to reach listingsPerItem.
		for i := len(valid); i < listingsPerItem; i++ {
			if err := e.createListing(ctx, item, price, stackMax, orderExpiry); err != nil {
				log.Printf("listing %s: %v", item.TemplateID, err)
				errs++
			} else {
				created++
			}
		}
	}

	log.Printf("tick: %d created, %d topped up, %d pruned, %d errors", created, topped, pruned, errs)
}

func (e *Exchange) updatePrices(ctx context.Context, catalog []CatalogItem) {
	catalogMap := make(map[string]CatalogItem, len(catalog))
	for _, item := range catalog {
		catalogMap[item.TemplateID] = item
	}

	rows, err := e.db.Query(ctx, `
		SELECT o.template_id,
		       COALESCE(SUM(f.stack_size), 0)         AS sold,
		       COALESCE(MAX(s.initial_stack_size), 0) AS listed
		FROM dune.dune_exchange_orders o
		JOIN dune.dune_exchange_sell_orders s ON s.order_id = o.id
		LEFT JOIN dune.dune_exchange_fulfilled_orders f ON f.order_id = o.id
		WHERE o.owner_id = $1 AND o.is_npc_order = TRUE
		GROUP BY o.template_id`, e.ownerID)
	if err != nil {
		log.Printf("price stats: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tmpl string
		var sold, listed int64
		if err := rows.Scan(&tmpl, &sold, &listed); err != nil {
			continue
		}
		item, ok := catalogMap[tmpl]
		if !ok {
			continue
		}
		current := e.prices[tmpl]
		if current <= 0 {
			current = item.ListPrice
		}
		var frac float64
		if listed > 0 {
			frac = float64(sold) / float64(listed)
		}
		e.prices[tmpl] = adjustPrice(item, current, frac)
	}
}
