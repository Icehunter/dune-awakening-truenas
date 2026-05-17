package main

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
)

type CatalogItem struct {
	TemplateID  string
	DisplayName string
	StackMax    int64
	Volume      float64
	Tier        int
	Rarity      string
	BasePrice   int64
	Category    string // e.g. "items/misc/refinedresources"
	ListPrice   int64
	IsSchematic bool
}

type itemNameEntry struct {
	ID   string `json:"ID"`
	Name struct {
		En string `json:"en"`
	} `json:"name"`
}

type itemDataEntry struct {
	Name        string  `json:"name"`
	StackMax    int64   `json:"stack_max"`
	Volume      float64 `json:"volume"`
	Tier        int     `json:"tier"`
	Rarity      string  `json:"rarity"`
	BasePrice   int64   `json:"vendor_price"`
	Category    string  `json:"category"`
	Tradeable   *bool   `json:"tradeable"`
	IsSchematic bool    `json:"is_schematic"`
}

type itemDataFile struct {
	Items map[string]itemDataEntry `json:"items"`
}

var (
	flagItemData  = flag.String("itemdata", "../dune-admin/item-data.json", "path to item-data.json")
	flagItemNames = flag.String("itemnames", "../dune-admin/dune-item-names.json", "path to dune-item-names.json")
)

func loadCatalog() ([]CatalogItem, error) {
	namesRaw, err := os.ReadFile(*flagItemNames)
	if err != nil {
		return nil, err
	}
	var names []itemNameEntry
	if err := json.Unmarshal(namesRaw, &names); err != nil {
		return nil, err
	}

	dataRaw, err := os.ReadFile(*flagItemData)
	if err != nil {
		return nil, err
	}
	var dataFile itemDataFile
	if err := json.Unmarshal(dataRaw, &dataFile); err != nil {
		return nil, err
	}

	// Build a set of IDs already covered by the names list.
	seenIDs := make(map[string]bool, len(names))

	var catalog []CatalogItem
	for _, n := range names {
		if strings.HasPrefix(n.ID, "Emote_") {
			continue
		}
		seenIDs[n.ID] = true

		item := CatalogItem{
			TemplateID:  n.ID,
			DisplayName: n.Name.En,
		}

		if d, ok := dataFile.Items[n.ID]; ok {
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
			item.StackMax = d.StackMax
			item.Volume = d.Volume
			item.Tier = d.Tier
			item.Rarity = d.Rarity
			item.BasePrice = d.BasePrice
			item.Category = d.Category
			item.IsSchematic = d.IsSchematic
			if item.DisplayName == "" {
				item.DisplayName = d.Name
			}
		}

		item.ListPrice = computePrice(item)
		catalog = append(catalog, item)
	}

	// Add schematics from item-data.json that aren't in dune-item-names.json.
	// The CDN names list doesn't include these, but they were merged in by
	// build-schematic-data.sh.
	for id, d := range dataFile.Items {
		if seenIDs[id] || !d.IsSchematic {
			continue
		}
		if d.Tradeable != nil && !*d.Tradeable {
			continue
		}
		item := CatalogItem{
			TemplateID:  id,
			DisplayName: d.Name,
			StackMax:    d.StackMax,
			Volume:      d.Volume,
			Tier:        d.Tier,
			Rarity:      d.Rarity,
			BasePrice:   d.BasePrice,
			Category:    d.Category,
			IsSchematic: true,
		}
		item.ListPrice = computePrice(item)
		catalog = append(catalog, item)
	}

	return catalog, nil
}
