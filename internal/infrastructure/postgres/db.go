// Package postgres implements the repository interfaces from
// internal/domain/repository against Postgres, and is the only package
// under internal/infrastructure that database/sql-shaped code lives in —
// nothing in application/ imports database/sql or *sql.DB directly.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Open connects to Postgres and verifies the connection with a bounded-time
// ping, so a bad POSTGRES_URL fails at startup (docs/09-deployment.md#configuration-strategy)
// rather than on the first query.
func Open(url string) (*sql.DB, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return db, nil
}
