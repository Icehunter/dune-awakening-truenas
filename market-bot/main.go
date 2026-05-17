package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	flagDBHost       = flag.String("dbhost", "", "PostgreSQL host (required)")
	flagDBPort       = flag.Int("dbport", 15432, "PostgreSQL port")
	flagDBUser       = flag.String("dbuser", "dune", "PostgreSQL user")
	flagDBPass       = flag.String("dbpass", "dune", "PostgreSQL password")
	flagDBName       = flag.String("dbname", "dune", "PostgreSQL database")
	flagCacheDB      = flag.String("cachedb", "/data/market-bot-cache.db", "SQLite path for category cache")
	flagInterval     = flag.Duration("interval", 5*time.Minute, "restock tick interval")
	flagBuyThreshold = flag.Float64("buythreshold", 1.05, "buy player listings at or below this multiple of the bot's sell price (0 = disable buying)")
	flagMaxBuys      = flag.Int("maxbuys", 50, "max player listings to purchase per tick")
)

func main() {
	flag.Parse()

	if *flagDBHost == "" {
		fmt.Fprintln(os.Stderr, "error: -dbhost is required")
		flag.Usage()
		os.Exit(1)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("market-bot ")

	ctx := context.Background()

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		*flagDBHost, *flagDBPort, *flagDBUser, *flagDBPass, *flagDBName,
	)
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}
	log.Printf("connected to %s:%d/%s", *flagDBHost, *flagDBPort, *flagDBName)

	log.Println("loading catalog...")
	catalog, err := loadCatalog()
	if err != nil {
		log.Fatalf("load catalog: %v", err)
	}
	log.Printf("catalog: %d listable items", len(catalog))

	ex, err := NewExchange(pool, *flagCacheDB, catalog)
	if err != nil {
		log.Fatalf("init exchange: %v", err)
	}
	ex.buyThreshold = *flagBuyThreshold
	ex.maxBuys = *flagMaxBuys

	log.Println("initializing exchange...")
	if err := ex.Init(ctx, catalog); err != nil {
		log.Fatalf("init: %v", err)
	}
	log.Println("exchange ready")

	// First tick immediately, then on interval.
	ex.Tick(ctx, catalog)

	ticker := time.NewTicker(*flagInterval)
	defer ticker.Stop()
	for range ticker.C {
		ex.Tick(ctx, catalog)
	}
}
