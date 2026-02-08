// Package db provides database connection and management for axon-server.
package db

import (
	"context"
	"embed"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migration represents a database migration.
type Migration struct {
	Name string
	SQL  string
}

// Migrate runs all pending database migrations.
func (db *DB) Migrate(ctx context.Context) error {
	// Create migrations table if not exists
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			name VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of applied migrations
	var applied []string
	err = db.SelectContext(ctx, &applied, "SELECT name FROM _migrations ORDER BY name")
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}
	appliedSet := make(map[string]bool)
	for _, name := range applied {
		appliedSet[name] = true
	}

	// Get list of migration files
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if appliedSet[m.Name] {
			log.Printf("Migration %s already applied, skipping", m.Name)
			continue
		}

		log.Printf("Applying migration %s...", m.Name)
		if err := db.applyMigration(ctx, m); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", m.Name, err)
		}
		log.Printf("Migration %s applied successfully", m.Name)
	}

	return nil
}

func (db *DB) applyMigration(ctx context.Context, m Migration) error {
	return db.Transaction(ctx, func(tx *sqlx.Tx) error {
		// Execute migration SQL
		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			return fmt.Errorf("failed to execute migration SQL: %w", err)
		}

		// Record migration as applied
		_, err := tx.ExecContext(ctx,
			"INSERT INTO _migrations (name) VALUES ($1)",
			m.Name,
		)
		if err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}

		return nil
	})
}

func loadMigrations() ([]Migration, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read migration %s: %w", entry.Name(), err)
		}

		migrations = append(migrations, Migration{
			Name: entry.Name(),
			SQL:  string(content),
		})
	}

	// Sort by name to ensure consistent order
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})

	return migrations, nil
}
