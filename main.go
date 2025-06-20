package main

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"os"
)

//go:embed templates/*.html
var templatesFS embed.FS

var templates = template.Must(
	template.ParseFS(templatesFS, "templates/*.html"),
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

	address := getEnv("ADDRESS", ":8080")

	mux := http.NewServeMux()
	mux.Handle("/", indexHandler(db))
	mux.HandleFunc("/lookup/", lookupHandler)
	mux.Handle("/top", topHandler(db))

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
