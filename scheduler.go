package main

import (
	"context"
	"database/sql"
	"log/slog"
	"math/rand"
	"time"
)

func startScheduler(ctx context.Context, db *sql.DB) error {
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM domains",
	).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	rand.Seed(time.Now().UnixNano())
	durStr := getEnv("CHECK_INTERVAL", "")
	var d time.Duration
	if durStr != "" {
		if v, err := time.ParseDuration(durStr); err == nil {
			d = v
		}
	}
	if d == 0 {
		d = time.Duration(int64(24*time.Hour) / int64(count))
	}
	if d <= 0 {
		d = time.Minute
	}
	t := time.NewTicker(d)
	go schedulerLoop(ctx, db, t)
	return nil
}

func schedulerLoop(ctx context.Context, db *sql.DB, t *time.Ticker) {
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			id, name, err := nextDomain(ctx, db)
			if err != nil {
				slog.Error("next", "err", err)
				continue
			}
			if err := checkDomain(ctx, db, id, name); err != nil {
				slog.Error("check", "err", err, "domain", name)
			}
		}
	}
}

func nextDomain(ctx context.Context, db *sql.DB) (int, string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT d.id, d.name
                FROM domains d
                LEFT JOIN (
                        SELECT domain_id,
                               MAX(checked_at) AS last_check
                        FROM dns_checks
                        GROUP BY domain_id
                ) c ON d.id = c.domain_id
                ORDER BY COALESCE(last_check, '1970-01-01') ASC,
                         d.rank ASC
                LIMIT 5`)
	if err != nil {
		return 0, "", err
	}
	defer rows.Close()
	var ids []int
	var names []string
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return 0, "", err
		}
		ids = append(ids, id)
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return 0, "", err
	}
	if len(ids) == 0 {
		return 0, "", sql.ErrNoRows
	}
	i := rand.Intn(len(ids))
	return ids[i], names[i], nil
}

func checkDomain(ctx context.Context, db *sql.DB, id int, name string) error {
	records, err := lookupDS(ctx, name)
	has := false
	errStr := ""
	if err != nil {
		errStr = err.Error()
	} else {
		has = len(records) > 0
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO dns_checks(domain_id, has_dnssec, error)
                VALUES(?, ?, ?)`,
		id, has, errStr)
	return err
}
