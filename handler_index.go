package main

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"
)

type domainRow struct {
	Rank          int
	Name          string
	Base          string
	TLD           string
	Important     bool
	HasDNSSEC     bool
	CheckedAt     string
	CheckedAtTime time.Time
}

func dnssecRatio(ctx context.Context, db *sql.DB, limit int) (float64, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*)
                 FROM domains d
                 LEFT JOIN dns_checks c ON c.id = (
                     SELECT id FROM dns_checks dc
                     WHERE dc.domain_id = d.id
                     ORDER BY dc.checked_at DESC LIMIT 1
                 )
                 WHERE d.rank <= ? AND c.has_dnssec = 1`,
		limit,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return 100 * float64(count) / float64(limit), nil
}

func indexHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			i, err := strconv.Atoi(p)
			if err == nil && i > 0 {
				page = i
			}
		}
		const perPage = 50
		offset := (page - 1) * perPage
		rows, err := db.Query(
			`SELECT d.rank, d.name, c.has_dnssec, c.checked_at
                         FROM domains d
                         LEFT JOIN dns_checks c ON c.id = (
                             SELECT id FROM dns_checks dc
                             WHERE dc.domain_id = d.id
                             ORDER BY dc.checked_at DESC LIMIT 1
                         )
                         ORDER BY d.rank
                         LIMIT ? OFFSET ?`,
			perPage+1, offset,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		list := make([]domainRow, 0, perPage)
		for rows.Next() {
			var rec domainRow
			var checked sql.NullTime
			var sec sql.NullBool
			if err := rows.Scan(&rec.Rank, &rec.Name, &sec, &checked); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			rec.Base, rec.TLD = domainParts(rec.Name)
			rec.Important = isImportantTLD(rec.TLD)
			rec.HasDNSSEC = sec.Valid && sec.Bool
			if checked.Valid {
				rec.CheckedAtTime = checked.Time
				rec.CheckedAt = checked.Time.Format("2006-01-02 15:04")
			}
			list = append(list, rec)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		hasNext := len(list) > perPage
		if hasNext {
			list = list[:perPage]
		}
		p1000, err := dnssecRatio(r.Context(), db, 1000)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		p500, err := dnssecRatio(r.Context(), db, 500)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		p100, err := dnssecRatio(r.Context(), db, 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data := struct {
			Domains  []domainRow
			PrevPage int
			NextPage int
			Page     int
			Pct1000  float64
			Pct500   float64
			Pct100   float64
		}{
			Domains: list,
			Page:    page,
			Pct1000: p1000,
			Pct500:  p500,
			Pct100:  p100,
		}
		if page > 1 {
			data.PrevPage = page - 1
		}
		if hasNext {
			data.NextPage = page + 1
		}
		err = templates.ExecuteTemplate(w, "index", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}
