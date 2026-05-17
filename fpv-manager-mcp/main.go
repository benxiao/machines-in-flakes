package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	ctx := context.Background()

	dsn := os.Getenv("FPV_DB_DSN")
	if dsn == "" {
		dsn = "host=/run/postgresql dbname=fpv_manager user=fpv_manager sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	q := &Queries{db: pool}
	s := server.NewMCPServer("fpv-manager", "1.0.0")
	registerTools(s, q)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
