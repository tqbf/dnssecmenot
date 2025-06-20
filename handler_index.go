package main

import (
	"database/sql"
	"net/http"
	"strconv"
)

type domainRow struct {
	Rank      int
	Name      string
	HasDNSSEC bool
	CheckedAt string
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
			rec.HasDNSSEC = sec.Valid && sec.Bool
			if checked.Valid {
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
