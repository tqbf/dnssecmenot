package main

import (
	"context"
	"net/http"

	"github.com/miekg/dns"
)

func lookupHandler(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Path[len("/lookup/"):]
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
