package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

type reportRow struct {
	Template string
	Sold     int64
	Listed   int64
	SellPct  float64
	Price    int64
	MinPrice int64
	MaxPrice int64
	Buyable  bool
}

// runReport prints a TSV analytics summary of all bot listings and exits.
// Columns: template, sold, listed, sell_pct, current_price, min_price, max_price, buyable
func runReport(ctx context.Context, pool *pgxpool.Pool, ex *Exchange, catalog []CatalogItem) {
	catalogMap := make(map[string]CatalogItem, len(catalog))
	for _, item := range catalog {
		catalogMap[item.TemplateID] = item
	}

	rows, err := pool.Query(ctx, `
		SELECT o.template_id,
		       COALESCE(SUM(f.stack_size), 0)          AS sold,
		       COALESCE(MAX(s.initial_stack_size), 0)  AS listed
		FROM dune.dune_exchange_orders o
		JOIN dune.dune_exchange_sell_orders s ON s.order_id = o.id
		LEFT JOIN dune.dune_exchange_fulfilled_orders f ON f.order_id = o.id
		WHERE o.owner_id = $1 AND o.is_npc_order = TRUE
		GROUP BY o.template_id
		ORDER BY o.template_id`, ex.ownerID)
	if err != nil {
		fmt.Printf("error querying stats: %v\n", err)
		return
	}
	defer rows.Close()

	var report []reportRow
	for rows.Next() {
		var tmpl string
		var sold, listed int64
		if err := rows.Scan(&tmpl, &sold, &listed); err != nil {
			continue
		}
		var sellPct float64
		if listed > 0 {
			sellPct = float64(sold) / float64(listed) * 100
		}
		price := ex.prices[tmpl]
		item := catalogMap[tmpl]
		report = append(report, reportRow{
			Template: tmpl,
			Sold:     sold,
			Listed:   listed,
			SellPct:  sellPct,
			Price:    price,
			MinPrice: item.MinPrice,
			MaxPrice: item.MaxPrice,
			Buyable:  item.Buyable,
		})
	}

	// Also include catalog items that have never had a listing (price=0, sold=0).
	listed := make(map[string]bool, len(report))
	for _, r := range report {
		listed[r.Template] = true
	}
	for _, item := range catalog {
		if listed[item.TemplateID] {
			continue
		}
		report = append(report, reportRow{
			Template: item.TemplateID,
			Sold:     0,
			Listed:   0,
			SellPct:  0,
			Price:    item.ListPrice,
			MinPrice: item.MinPrice,
			MaxPrice: item.MaxPrice,
			Buyable:  item.Buyable,
		})
	}

	sort.Slice(report, func(i, j int) bool {
		return report[i].Template < report[j].Template
	})

	// Header
	fmt.Println("template\tsold\tlisted\tsell_pct\tcurrent_price\tmin_price\tmax_price\tbuyable")
	for _, r := range report {
		minP := "-"
		if r.MinPrice > 0 {
			minP = fmt.Sprintf("%d", r.MinPrice)
		}
		maxP := "-"
		if r.MaxPrice > 0 {
			maxP = fmt.Sprintf("%d", r.MaxPrice)
		}
		fmt.Printf("%s\t%d\t%d\t%.1f%%\t%d\t%s\t%s\t%v\n",
			r.Template, r.Sold, r.Listed, r.SellPct, r.Price, minP, maxP, r.Buyable)
	}
}
