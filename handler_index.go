package main

import (
	"context"
	"database/sql"
	"errors"
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
	Class         string
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

func (srv *DNSSECMeNot) handleIndex(w http.ResponseWriter, r *http.Request) {
	hx := r.Header.Get("HX-Request") == "true"
	trigger := r.Header.Get("HX-Trigger")
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		i, err := strconv.Atoi(p)
		if err == nil && i > 0 {
			page = i
		}
	}
	const perPage = 50
	offset := (page - 1) * perPage
	rows, err := srv.db.Query(
		`SELECT d.rank, d.name, d.class,
                               c.has_dnssec, c.checked_at
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

	// assemble the page data set
	list := make([]domainRow, 0, perPage)
	for rows.Next() {
		var (
			rec     domainRow
			class   sql.NullString
			sec     sql.NullBool
			checked sql.NullTime
		)
		if err := rows.Scan(&rec.Rank, &rec.Name, &class, &sec, &checked); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rec.Base, rec.TLD = domainParts(rec.Name)
		rec.Important = isImportantTLD(rec.TLD)
		if class.Valid {
			rec.Class = class.String
		}
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
	p1000, err1 := dnssecRatio(r.Context(), srv.db, 1000)
	p500, err2 := dnssecRatio(r.Context(), srv.db, 500)
	p100, err3 := dnssecRatio(r.Context(), srv.db, 100)
	if err = errors.Join(err1, err2, err3); err != nil {
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

	tpl := "index"
	if hx {
		if trigger == "more-table" {
			tpl = "rowsTable"
		} else if trigger == "more-mobile" {
			tpl = "rowsMobile"
		}
	}

	err = templates.ExecuteTemplate(w, tpl, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
