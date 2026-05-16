package main

import (
	"fmt"
	"testing"
)

func TestUniqueSchematicsMask(t *testing.T) {
	cases := []struct {
		category string
		wantMask int32
	}{
		// GARMENTS → UNIQUE SCHEMATICS(5) → lightarmor(0)/heavyarmor(1)/stillsuits(2)
		{"items/garment/lightarmor/chest", 0x00050000},
		{"items/garment/heavyarmor/head", 0x00050100},
		{"items/garment/stillsuits/hands", 0x00050200},
		{"items/garment/socialwearables/chest", 0x00050400},
		// WEAPONS → UNIQUE SCHEMATICS(3) → shortblades(0)/longblades(1)/pistol(2)/battlerifle(3)
		{"items/weapons/shortblades", 0x01030000},
		{"items/weapons/longblades", 0x01030100},
		{"items/weapons/sidearms/pistol", 0x01030200},
		{"items/weapons/rifles/battlerifle", 0x01030300},
		{"items/weapons/rifles/spitdart", 0x01030600},
		// Non-remapped categories stay standard (ok=false)
	}

	for _, tc := range cases {
		mask, depth, ok := UniqueSchematicsMask(tc.category)
		if !ok {
			t.Errorf("%-45s: got ok=false, want mask=0x%08X", tc.category, uint32(tc.wantMask))
			continue
		}
		if mask != tc.wantMask {
			t.Errorf("%-45s: got=0x%08X depth=%d want=0x%08X", tc.category, uint32(mask), depth, uint32(tc.wantMask))
		} else {
			fmt.Printf("OK %-45s 0x%08X depth=%d\n", tc.category, uint32(mask), depth)
		}
	}

	// Categories without UNIQUE SCHEMATICS section should return ok=false
	for _, cat := range []string{"items/augment/ranged", "items/misc/components", "items/vehicles/sandbike/chassis"} {
		if _, _, ok := UniqueSchematicsMask(cat); ok {
			t.Errorf("%s: expected ok=false (no unique schematics remapping)", cat)
		}
	}
}

func TestCategoryMask(t *testing.T) {
	cases := []struct {
		category string
		wantD1   byte
		wantD2   byte
		wantD3   byte
		wantMask int32
	}{
		// Weapons: shortblades remapped to items/weapons/melee/shortblades
		{"items/weapons/shortblades", 1, 0, 0, 0x01000000},
		{"items/weapons/longblades", 1, 0, 1, 0x01000100},
		// MISC
		{"items/misc/refinedresources", 5, 1, 0, 0x05010000},
		{"items/misc/components", 5, 2, 0, 0x05020000},
		{"items/misc/rawresources", 5, 3, 0, 0x05030000},
		// GARMENTS
		{"items/garment/lightarmor/chest", 0, 0, 1, 0x00000100},
		{"items/garment/heavyarmor/head", 0, 1, 0, 0x00010000},
		// VEHICLES
		{"items/vehicles/mediumornithopter/chassis", 2, 3, 0, 0x02030000},
		// UTILITY
		{"items/utility/consumables", 3, 6, 0, 0x03060000},
		{"items/utility/gatheringtools/cutteray", 3, 3, 0, 0x03030000},
	}

	catalog := make([]CatalogItem, len(cases))
	for i, tc := range cases {
		catalog[i] = CatalogItem{Category: tc.category}
	}
	idx := buildSegmentIndex(catalog)

	for _, tc := range cases {
		mask, _ := CategoryMask(tc.category, idx)
		d1 := byte((mask >> 24) & 0xFF)
		d2 := byte((mask >> 16) & 0xFF)
		d3 := byte((mask >> 8) & 0xFF)
		d0 := byte(mask & 0xFF)
		if mask != tc.wantMask {
			t.Errorf("%-50s got=0x%08X want=0x%08X [d1=%d d2=%d d3=%d d0=%d]",
				tc.category, uint32(mask), uint32(tc.wantMask), d1, d2, d3, d0)
		} else {
			fmt.Printf("OK %-50s 0x%08X\n", tc.category, uint32(mask))
		}
	}
}
