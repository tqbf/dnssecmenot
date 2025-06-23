package main

import (
	"net/http"
	"time"
)

type changeRow struct {
	Name          string
	HasDNSSEC     bool
	CheckedAt     string
	CheckedAtTime time.Time
}

func (srv *DNSSECMeNot) handleChanges(w http.ResponseWriter, r *http.Request) {
	rows, err := srv.db.Query(`
                        SELECT d.name, c.checked_at, c.has_dnssec
                        FROM (
                                SELECT domain_id, checked_at, has_dnssec,
                                       LAG(has_dnssec) OVER (
                                               PARTITION BY domain_id
                                               ORDER BY checked_at
                                       ) AS prev
                                FROM dns_checks
                        ) c
                        JOIN domains d ON d.id = c.domain_id
                        WHERE c.prev IS NULL OR c.prev != c.has_dnssec
                        ORDER BY c.checked_at DESC
                        LIMIT 200`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	list := make([]changeRow, 0, 64)
	for rows.Next() {
		var rec changeRow
		if err := rows.Scan(&rec.Name, &rec.CheckedAtTime, &rec.HasDNSSEC); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rec.CheckedAt = rec.CheckedAtTime.Format("2006-01-02 15:04")
		list = append(list, rec)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct{ Changes []changeRow }{Changes: list}
	if err := templates.ExecuteTemplate(w, "changes", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
