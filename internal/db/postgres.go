package db

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/url"
	"path"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// sanitizeDatabaseURL removes unsupported query parameters (e.g. channel_binding)
// from the database URL to avoid pgx parse errors.
func sanitizeDatabaseURL(databaseURL string) string {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return databaseURL
	}
	q := u.Query()
	q.Del("channel_binding")
	u.RawQuery = q.Encode()
	return u.String()
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	databaseURL = sanitizeDatabaseURL(databaseURL)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		sql, err := migrationsFS.ReadFile(path.Join("migrations", entry.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("exec %s: %w", entry.Name(), err)
		}
		log.Printf("migration applied: %s", entry.Name())
	}
	return nil
}
