package main

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
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
	}).ParseFS(templatesFS, "templates/*.html"),
)

func main() {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := startScheduler(ctx, db); err != nil {
		slog.Error("scheduler", "err", err)
		os.Exit(1)
	}

	address := getEnv("ADDRESS", ":8080")

	mux := http.NewServeMux()
	mux.Handle("/", indexHandler(db))
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
