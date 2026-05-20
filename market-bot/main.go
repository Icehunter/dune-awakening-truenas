package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	flagDBHost       = flag.String("dbhost", "", "PostgreSQL host (required)")
	flagDBPort       = flag.Int("dbport", 15432, "PostgreSQL port")
	flagDBUser       = flag.String("dbuser", "dune", "PostgreSQL user")
	flagDBPass       = flag.String("dbpass", "dune", "PostgreSQL password")
	flagDBName       = flag.String("dbname", "dune", "PostgreSQL database")
	flagCacheDB      = flag.String("cachedb", "/data/market-bot-cache.db", "SQLite path for category cache")
	flagBuyInterval  = flag.Duration("buyinterval", 5*time.Minute, "how often to buy player listings")
	flagListInterval = flag.Duration("listinterval", 30*time.Minute, "how often to restock/prune bot listings")
	flagBuyThreshold = flag.Float64("buythreshold", 1.05, "buy player listings at or below this multiple of the bot's sell price (0 = disable buying)")
	flagMaxBuys      = flag.Int("maxbuys", 50, "max player listings to purchase per tick")
	flagReport       = flag.Bool("report", false, "print per-item sales analytics as TSV and exit (does not run the bot loop)")
)

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func main() {
	flag.Parse()

	if *flagDBHost == "" {
		fmt.Fprintln(os.Stderr, "error: -dbhost is required")
		flag.Usage()
		os.Exit(1)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("market-bot ")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		*flagDBHost, *flagDBPort, *flagDBUser, *flagDBPass, *flagDBName,
	)
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Fatalf("db config: %v", err)
	}
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET search_path TO dune, public`)
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
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

	// Report mode: print analytics and exit without running the bot loop.
	if *flagReport {
		runReport(ctx, pool, ex, catalog)
		return
	}

	// Run both immediately on start.
	ex.Tick(ctx, catalog)

	tick := time.NewTicker(minDuration(*flagBuyInterval, *flagListInterval))
	defer tick.Stop()
	nextBuy := time.Now().Add(*flagBuyInterval)
	nextList := time.Now().Add(*flagListInterval)
	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down (signal received)")
			return
		case now := <-tick.C:
			if now.After(nextBuy) {
				ex.BuyTick(ctx)
				nextBuy = now.Add(*flagBuyInterval)
			}
			if now.After(nextList) {
				ex.ListTick(ctx, catalog)
				nextList = now.Add(*flagListInterval)
			}
		}
	}
}
