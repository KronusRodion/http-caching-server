package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func Init(dbURL string) error {

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return fmt.Errorf("unable to parse database config: %w", err)
	}

	DB, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("unable to create connection pool: %w", err)
	}

	err = DB.Ping(context.Background())
	if err != nil {
		return fmt.Errorf("unable to ping database: %w", err)
	}

	log.Println("Successfully connected to database")

	if err := runMigrations(); err != nil {
		return fmt.Errorf("migrations failed: %w", err)
	}

	return nil
}

func CloseDB() {
	DB.Close()
}



func runMigrations() error {
	
	migrationsDir := filepath.Join("internal", "database", "migrations")
    migrationFiles := []string{
        filepath.Join(migrationsDir, "tokens.sql"),
    }

	for _, file := range migrationFiles {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		sql := string(sqlBytes)
		// Разделяем на отдельные запросы (если файл содержит несколько)
		queries := strings.Split(sql, ";")

		for _, query := range queries {
			query = strings.TrimSpace(query)
			if query == "" {
				continue
			}

			if _, err := DB.Exec(context.Background(), query); err != nil {
				return fmt.Errorf("failed to execute migration from %s: %w\nQuery: %s", file, err, query)
			}
		}

		log.Printf("Successfully executed migrations from %s", file)
	}

	return nil
}