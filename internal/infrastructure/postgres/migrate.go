package postgres

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations exécute tous les fichiers *.up.sql du dossier migrationsDir dans l'ordre.
// Crée la table schema_migrations si elle n'existe pas.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("cannot create schema_migrations: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, f := range files {
		version := strings.TrimSuffix(filepath.Base(f), ".up.sql")

		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version,
		).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		sql, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("cannot read migration %s: %w", f, err)
		}

		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %s failed: %w", version, err)
		}

		if _, err := pool.Exec(ctx,
			`INSERT INTO schema_migrations(version) VALUES($1)`, version,
		); err != nil {
			return err
		}

		log.Printf("migration applied: %s", version)
	}

	return nil
}
