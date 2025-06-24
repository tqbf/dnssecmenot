package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"

	"github.com/miekg/dns"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*.css
var staticFS embed.FS

var templates = template.Must(
	template.New("").Funcs(template.FuncMap{
		"relativeTime": relativeTime,
		"classColor":   classColor,
	}).ParseFS(templatesFS, "templates/*.html"),
)

type DNSSECMeNot struct {
	db *sql.DB
}

func main() {
	h := slog.NewTextHandler(os.Stderr, nil)
	slog.SetDefault(slog.New(h))

	var (
		updatePath = flag.String("update-classes", "", "load classes")
		listFlag   = flag.Bool("list-unclassed", false, "list domains")
	)
	flag.Parse()

	db, err := openDB( /* really should take the path arg here */ )
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := maybeSeedDomains(db); err != nil {
		slog.Error("seed", "err", err)
		os.Exit(1)
	}

	switch {
	case *updatePath != "":
		if err := loadClasses(db, *updatePath); err != nil {
			slog.Error("classes", "err", err)
			os.Exit(1)
		}
		return

	case *listFlag:
		if err := listUnclassed(db, os.Stdout); err != nil {
			slog.Error("list", "err", err)
			os.Exit(1)
		}
		return
	}

	// nope we're servering

	srv := &DNSSECMeNot{
		db: db,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := startScheduler(ctx, db); err != nil {
		slog.Error("scheduler", "err", err)
		os.Exit(1)
	}

	address := getEnv("ADDRESS", ":8080")

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(srv.handleIndex))
	mux.Handle("/changes", http.HandlerFunc(srv.handleChanges))
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))

	slog.Info("listening", "addr", address)
	if err := http.ListenAndServe(address, mux); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var resolvers = []string{
	"8.8.8.8:53",
	"1.1.1.1:53",
	"9.9.9.9:53",
}

func pickResolvers() (string, string) {
	n := len(resolvers)
	i := rand.IntN(n)
	j := rand.IntN(n - 1)
	if j >= i {
		j++
	}
	return resolvers[i], resolvers[j]
}

func lookupDS(ctx context.Context, domain string) ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeDS)

	c := new(dns.Client)
	r1, r2 := pickResolvers()

	a, _, err1 := c.ExchangeContext(ctx, m, r1)
	b, _, err2 := c.ExchangeContext(ctx, m, r2)

	if err1 != nil || err2 != nil {
		return nil, errors.Join(err1, err2)
	}

	pa := len(a.Answer) > 0
	pb := len(b.Answer) > 0
	if pa != pb {
		return nil, fmt.Errorf("mismatch")
	}
	if !pa {
		return nil, nil
	}
	return a.Answer, nil
}

func loadClasses(db *sql.DB, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open json %s: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	var m map[string][]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		"UPDATE domains SET class = ? WHERE name = ?",
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for class, list := range m {
		for _, name := range list {
			if _, err := stmt.Exec(class, name); err != nil {
				return fmt.Errorf("set class tx: %w", err)
			}

			slog.Info("set class", "domain", name, "class", class)
		}
	}
	return tx.Commit()
}

func listUnclassed(db *sql.DB, w io.Writer) error {
	const q = `
                SELECT name FROM domains
                WHERE (class IS NULL OR class = '')
                AND rank <= 1000
                ORDER BY rank`
	rows, err := db.Query(q)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		fmt.Fprintln(w, name)
	}

	return rows.Err()
}
