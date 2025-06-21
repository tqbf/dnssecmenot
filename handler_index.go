package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type domainRow struct {
	Rank       int
	Name       string
	HasDNSSEC  bool
	CheckedAt  string
	CheckedAgo string
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
		now := time.Now()
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
			rec.HasDNSSEC = sec.Valid && sec.Bool
			if checked.Valid {
				rec.CheckedAt = checked.Time.Format("2006-01-02 15:04")
				rec.CheckedAgo = relativeTime(now, checked.Time)
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
		data := struct {
			Domains  []domainRow
			PrevPage int
			NextPage int
			Page     int
		}{
			Domains: list,
			Page:    page,
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

func relativeTime(now, t time.Time) string {
	if now.Year() == t.Year() && now.YearDay() == t.YearDay() {
		d := now.Sub(t)
		if d < time.Hour {
			m := int(d.Minutes())
			if m == 1 {
				return "1 minute ago"
			}
			return fmt.Sprintf("%d minutes ago", m)
		}
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	}
	y := now.AddDate(0, 0, -1)
	if y.Year() == t.Year() && y.YearDay() == t.YearDay() {
		return "yesterday"
	}
	days := int(now.Sub(t).Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}
