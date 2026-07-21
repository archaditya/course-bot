package postgres

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RunMigrations applies every *.up.sql file in migrationsDir that isn't
// already recorded in schema_migrations, in filename order, each inside its
// own transaction. This runs automatically on every `go run cmd/api/main.go`
// (see cmd/api/main.go) rather than requiring a separate migrate CLI step —
// so a fresh checkout with an empty database is fully bootstrapped by
// starting the API once.
//
// Deliberately hand-rolled instead of pulling in golang-migrate: the file
// naming convention (NNNNNN_description.up.sql / .down.sql) is the same one
// golang-migrate uses, so swapping to that library later — e.g. once
// down-migrations need CLI-driven rollback in production — is a drop-in
// change, not a schema rewrite.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     text PRIMARY KEY,
			applied_at  timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("postgres: creating schema_migrations table: %w", err)
	}

	files, err := upMigrationFiles(migrationsDir)
	if err != nil {
		return err
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	for _, f := range files {
		version := migrationVersion(f)
		if applied[version] {
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			return fmt.Errorf("postgres: reading migration %s: %w", f, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("postgres: starting migration transaction for %s: %w", f, err)
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("postgres: applying migration %s: %w", f, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			tx.Rollback()
			return fmt.Errorf("postgres: recording migration %s: %w", f, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("postgres: committing migration %s: %w", f, err)
		}
	}

	return nil
}

func upMigrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("postgres: reading migrations dir %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func appliedVersions(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("postgres: reading applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("postgres: scanning applied migration: %w", err)
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func migrationVersion(filename string) string {
	return strings.TrimSuffix(filename, ".up.sql")
}
