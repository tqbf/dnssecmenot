package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/miekg/dns"
)

func main() {
	checkTop := flag.Bool(
		"check-top",
		false,
		"check DS for top 1000 then exit",
	)
	flag.Parse()

	h := slog.NewTextHandler(os.Stderr, nil)
	slog.SetDefault(slog.New(h))
	db, err := openDB()
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := maybeSeedDomains(db); err != nil {
		slog.Error("seed", "err", err)
		os.Exit(1)
	}
	if *checkTop {
		if err := checkTopDomains(context.Background(), db); err != nil {
			slog.Error("check top", "err", err)
			os.Exit(1)
		}
		return
	}

	address := getEnv("ADDRESS", ":8080")

	mux := http.NewServeMux()
	mux.HandleFunc("/", helloHandler)
	mux.HandleFunc("/lookup/", lookupHandler)
	mux.Handle("/top", topHandler(db))

	slog.Info("listening", "addr", address)
	if err := http.ListenAndServe(address, mux); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("dnssecmenot"))
}

func topHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(
			"SELECT name FROM domains ORDER BY rank LIMIT 500",
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, "%s\n", name)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func lookupHandler(w http.ResponseWriter, r *http.Request) {
	// expects /lookup/example.com
	domain := r.URL.Path[len("/lookup/"):] // simple path parsing
	if domain == "" {
		http.Error(w, "missing domain", http.StatusBadRequest)
		return
	}

	records, err := lookupDS(r.Context(), domain)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(records) == 0 {
		w.Write([]byte("no DS records found\n"))
		return
	}
	for _, rr := range records {
		w.Write([]byte(rr.String() + "\n"))
	}
}

func lookupDS(ctx context.Context, domain string) ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeDS)

	c := new(dns.Client)
	r, _, err := c.ExchangeContext(ctx, m, "8.8.8.8:53")
	if err != nil {
		return nil, err
	}
	return r.Answer, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func checkTopDomains(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(
		ctx,
		"SELECT id, name FROM domains ORDER BY rank LIMIT 1000",
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for rows.Next() {
		var (
			id   int64
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		<-ticker.C
		slog.Info("checking", "domain", name)
		rrs, err := lookupDS(ctx, name)
		has := err == nil && len(rrs) > 0
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		if err := insertCheck(db, id, has, msg); err != nil {
			return err
		}
		slog.Info(
			"checked",
			"domain",
			name,
			"records",
			len(rrs),
			"error",
			msg,
		)
	}
	return rows.Err()
}

func insertCheck(db *sql.DB, id int64, ok bool, errMsg string) error {
	_, err := db.Exec(
		`INSERT INTO dns_checks(domain_id, has_dnssec, error)
                VALUES(?, ?, ?)`,
		id,
		ok,
		errMsg,
	)
	return err
}
