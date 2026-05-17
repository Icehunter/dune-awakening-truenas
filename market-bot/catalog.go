package main

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
)

type CatalogItem struct {
	TemplateID   string
	DisplayName  string
	StackMax     int64
	Volume       float64
	Tier         int
	Rarity       string
	BasePrice    int64
	Category     string // e.g. "items/misc/refinedresources"
	ListPrice    int64
	IsSchematic          bool
	MaterialCost         int64
	MaterialCostPerGrade [6]int64
	IsGradeable          bool
	MinQualityLevel      int
}

type itemDataEntry struct {
	Name         string  `json:"name"`
	StackMax     int64   `json:"stack_max"`
	Volume       float64 `json:"volume"`
	Tier         int     `json:"tier"`
	Rarity       string  `json:"rarity"`
	BasePrice    int64   `json:"vendor_price"`
	Category     string  `json:"category"`
	Tradeable    *bool   `json:"tradeable"`
	IsSchematic          bool     `json:"is_schematic"`
	MaterialCost         int64    `json:"material_cost"`
	MaterialCostPerGrade [6]int64 `json:"material_cost_per_grade"`
	IsGradeable          bool     `json:"is_gradeable"`
	MinQualityLevel      int      `json:"min_quality_level"`
}

type itemDataFile struct {
	Items map[string]itemDataEntry `json:"items"`
}

var (
	flagItemData = flag.String("itemdata", "../dune-admin/item-data.json", "path to item-data.json")
)

func loadCatalog() ([]CatalogItem, error) {
	dataRaw, err := os.ReadFile(*flagItemData)
	if err != nil {
		return nil, err
	}
	var dataFile itemDataFile
	if err := json.Unmarshal(dataRaw, &dataFile); err != nil {
		return nil, err
	}

	var catalog []CatalogItem
	for id, d := range dataFile.Items {
		if strings.HasPrefix(id, "Emote_") {
			continue
		}
		// Skip items explicitly excluded from the exchange.
		if d.Tradeable != nil && !*d.Tradeable {
			continue
		}
		// Skip items with no market category (e.g. reputation tokens).
		if d.Category == "" {
			continue
		}
		// Skip categories that are non-tradeable on the exchange
		// despite not having ExcludeFromExchange tags in raw data.
		if strings.HasPrefix(d.Category, "items/customization/") ||
			strings.HasPrefix(d.Category, "items/construction/") {
			continue
		}

		item := CatalogItem{
			TemplateID:           id,
			DisplayName:          d.Name,
			StackMax:             d.StackMax,
			Volume:               d.Volume,
			Tier:                 d.Tier,
			Rarity:               d.Rarity,
			BasePrice:            d.BasePrice,
			Category:             d.Category,
			IsSchematic:          d.IsSchematic,
			MaterialCost:         d.MaterialCost,
			MaterialCostPerGrade: d.MaterialCostPerGrade,
			IsGradeable:          d.IsGradeable,
			MinQualityLevel:      d.MinQualityLevel,
		}
		item.ListPrice = computePrice(item)
		catalog = append(catalog, item)
	}

	return catalog, nil
}
