// Command migrate applies the SQL files in services/core/migrations to the database
// named by DATABASE_URL, tracking applied filenames in a schema_migrations table so
// reruns are safe (already-applied files are skipped).
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	dir := "migrations"
	if v := os.Getenv("MIGRATIONS_DIR"); v != "" {
		dir = v
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	applied, err := run(context.Background(), db, dir)
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}
	fmt.Printf("migrations applied: %d\n", applied)
}

func run(ctx context.Context, db *sql.DB, dir string) (int, error) {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return 0, fmt.Errorf("ensure schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read migrations dir %q: %w", dir, err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	applied := 0
	for _, name := range files {
		alreadyApplied, err := isApplied(ctx, db, name)
		if err != nil {
			return applied, fmt.Errorf("check %s: %w", name, err)
		}
		if alreadyApplied {
			continue
		}

		if err := applyMigration(ctx, db, dir, name); err != nil {
			return applied, err
		}
		log.Printf("applied migration %s", name)
		applied++
	}

	return applied, nil
}

func isApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version,
	).Scan(&exists)
	return exists, err
}

func applyMigration(ctx context.Context, db *sql.DB, dir, name string) error {
	sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", name, err)
	}

	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply %s: %w", name, err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record %s: %w", name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", name, err)
	}

	return nil
}
