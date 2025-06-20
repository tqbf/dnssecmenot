package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/miekg/dns"
)

func main() {
	db, err := openDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	address := getEnv("ADDRESS", ":8080")

	mux := http.NewServeMux()
	mux.HandleFunc("/", helloHandler)
	mux.HandleFunc("/lookup/", lookupHandler)

	log.Printf("listening on %s", address)
	if err := http.ListenAndServe(address, mux); err != nil {
		log.Fatal(err)
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("dnssecmenot"))
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
