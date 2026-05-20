package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// ── config ────────────────────────────────────────────────────────────────────

var (
	sshHost         string
	sshUser         string
	sshKeyPath      string
	itemDataPath    string
	scripCurrencyID int
	dbPort          int
	dbUser          string
	dbPass          string
	dbName          string
	dbSchema        string
)

func init() {
	flag.StringVar(&sshHost, "host", "192.168.0.72:22", "SSH host:port")
	flag.StringVar(&sshUser, "user", "dune", "SSH user")
	flag.StringVar(&sshKeyPath, "key", "", "SSH private key path (auto-detected if empty)")
	flag.StringVar(&itemDataPath, "itemdata", "", "Item data JSON path (stack_max/volume overrides)")
	flag.IntVar(&scripCurrencyID, "scripcurrency", 1, "Scrip currency id (auto-detect if -1)")
	flag.IntVar(&dbPort, "dbport", 15432, "PostgreSQL port inside the cluster")
	flag.StringVar(&dbUser, "dbuser", "dune", "PostgreSQL user")
	flag.StringVar(&dbPass, "dbpass", "dune", "PostgreSQL password")
	flag.StringVar(&dbName, "dbname", "dune", "PostgreSQL database name")
	flag.StringVar(&dbSchema, "schema", "dune", "PostgreSQL schema")
}

func resolveKeyPath() string {
	if sshKeyPath != "" {
		return sshKeyPath
	}
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "dune"),
		"../sshKey",
		"./sshKey",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0]
}

func resolveItemDataPath() string {
	if itemDataPath != "" {
		return itemDataPath
	}
	candidates := []string{
		"./item-data.json",
		"../item-data.json",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func resolveItemNamesPath() string {
	candidates := []string{
		"./dune-item-names.json",
		"../dune-item-names.json",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

var itemData itemDataFile

func loadItemData() error {
	path := resolveItemDataPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read item data %s: %w", path, err)
	}
	var parsed itemDataFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parse item data %s: %w", path, err)
	}
	// Normalize keys to lowercase so lookups work regardless of whether the
	// DB template_id is PascalCase (MelangeSpice) or lowercase (melangespice).
	normalized := make(map[string]itemRule, len(parsed.Items))
	for k, v := range parsed.Items {
		normalized[strings.ToLower(k)] = v
	}
	parsed.Items = normalized
	itemData = parsed
	return nil
}

func loadItemNames() error {
	path := resolveItemNamesPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read item names %s: %w", path, err)
	}
	var raw []struct {
		ID   string            `json:"ID"`
		Name map[string]string `json:"name"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse item names %s: %w", path, err)
	}
	m := make(map[string]duneItemName, len(raw))
	for _, r := range raw {
		if r.ID == "" {
			continue
		}
		m[strings.ToLower(r.ID)] = duneItemName{ID: r.ID, Name: r.Name["en"]}
	}
	duneItemNames = m
	return nil
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()
	if err := loadItemData(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := loadItemNames(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if globalDB != nil {
		globalDB.Close()
	}
	if globalSSH != nil {
		globalSSH.Close()
	}
}
