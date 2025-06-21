package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand"
	"time"
)

func startScheduler(ctx context.Context, db *sql.DB) error {
	var (
		count  int
		durStr = getEnv("CHECK_INTERVAL", "") // see below
		d      time.Duration
	)

	if err := db.QueryRow(
		"SELECT COUNT(*) FROM domains",
	).Scan(&count); err != nil {
		return fmt.Errorf("count zones: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no zones in db")
	}

	// by default, scale to zones over a whole day of probes
	d = time.Duration(int64(24*time.Hour) / int64(count))

	// probably don't want to set this
	if durStr != "" {
		if v, err := time.ParseDuration(durStr); err == nil {
			d = v
		}
	}

	// don't piss off 8.8.8.8
	if d <= 0 {
		d = time.Minute
	}

	go schedulerLoop(ctx, db, d)
	return nil
}

func schedulerLoop(ctx context.Context, db *sql.DB, interval time.Duration) {
	t := time.NewTicker(interval)

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
	// subquery c: for each domain in dns_checks, get the most recent checked_at timestamp
	// join w/ domains on domain_id, left join to incl. zones w/ no checks
	// take 5, sorted by tranco rank, so we have some jitter and don't get stuck
	// on wacky corner cases
	rows, err := db.QueryContext(ctx, `
		SELECT d.id, d.name
		FROM domains d
		LEFT JOIN (
    		SELECT dc.domain_id,
           	MAX(dc.checked_at) AS last_check
            FROM dns_checks dc
            JOIN domains d2 ON dc.domain_id = d2.id
            WHERE d2.rank <= 1000
            GROUP BY dc.domain_id
        ) c ON d.id = c.domain_id
        ORDER BY COALESCE(last_check, '1970-01-01') ASC,
        d.rank ASC
        LIMIT 5`)
	if err != nil {
		return 0, "", fmt.Errorf("query next zones: %w", err)
	}
	defer rows.Close()

	var (
		ids   []int
		names []string
	)
	for rows.Next() {
		var (
			id   int
			name string
		)
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
		return 0, "", fmt.Errorf("query: %w", sql.ErrNoRows)
	}

	// jitter
	i := rand.Intn(len(ids))
	return ids[i], names[i], nil
}

func checkDomain(ctx context.Context, db *sql.DB, id int, name string) error {
	var (
		has    = false
		errStr = ""
	)

	slog.Info("checking", "domain", name)

	records, err := lookupDS(ctx, name)
	if err != nil {
		errStr = err.Error()
	} else {
		has = len(records) > 0
	}

	// i'm just going to not update
	// if nothing changes (just update the last check's timestamp)
	var (
		lastID  int
		lastHas sql.NullBool
		lastErr sql.NullString
	)
	err = db.QueryRowContext(ctx,
		`SELECT id, has_dnssec, error
                FROM dns_checks
                WHERE domain_id = ?
                ORDER BY checked_at DESC
                LIMIT 1`,
		id,
	).Scan(&lastID, &lastHas, &lastErr)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	var sameErr bool
	if lastErr.Valid {
		sameErr = lastErr.String == errStr
	} else {
		sameErr = errStr == ""
	}

	sameResult := err == nil &&
		lastHas.Valid &&
		lastHas.Bool == has &&
		sameErr

	if sameResult {
		_, err = db.ExecContext(ctx,
			`UPDATE dns_checks
	 SET checked_at = CURRENT_TIMESTAMP
	 WHERE id = ?`,
			lastID,
		)
		return err
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO dns_checks(domain_id, has_dnssec, error)
                VALUES(?, ?, ?)`,
		id, has, errStr,
	)
	return err
}
