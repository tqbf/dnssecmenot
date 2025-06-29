package main

import (
	"database/sql"
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func openDB() (*sql.DB, error) {
	path := getEnv("DB_PATH", "/data/app.db")
	dsn := fmt.Sprintf("%s?_txlock=immediate", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	for _, p := range []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA cache_size = 250000000;",
		"PRAGMA foreign_keys = true;",
		"PRAGMA temp_store = memory;",
	} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func applyMigrations(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		version := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		var count int
		if err := db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, version).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		content, err := migrationFiles.ReadFile(filepath.Join("migrations", e.Name()))
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("migration %s failed: %w", e.Name(), err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, version); err != nil {
			return err
		}
	}
	return nil
}

func maybeSeedDomains(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM domains").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	path := getEnv("DOMAINS_CSV", "tranco-5000.csv")
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open csv %s: %w", path, err)
	}
	defer file.Close()
	r := csv.NewReader(file)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare("INSERT INTO domains(name, rank) VALUES(?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(rec) < 2 {
			continue
		}
		rank, err := strconv.Atoi(strings.TrimSpace(rec[0]))
		if err != nil {
			return err
		}
		name := strings.TrimSpace(rec[1])
		if _, err := stmt.Exec(name, rank); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
