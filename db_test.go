package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedDomains(t *testing.T, db *sql.DB, n int) []string {
	t.Helper()
	file, err := os.Open("tranco-5000.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	r := csv.NewReader(file)
	stmt, err := db.Prepare(
		"INSERT INTO domains(name, rank) VALUES(?, ?)",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	names := make([]string, 0, n)
	for i := 0; i < n; i++ {
		rec, err := r.Read()
		if err != nil {
			t.Fatal(err)
		}
		rank, err := strconv.Atoi(strings.TrimSpace(rec[0]))
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSpace(rec[1])
		if _, err := stmt.Exec(name, rank); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

func insertCheck(
	t *testing.T, db *sql.DB, name string, ts time.Time, has bool,
) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO dns_checks(domain_id, checked_at, has_dnssec, error)
         VALUES((SELECT id FROM domains WHERE name = ?), ?, ?, '')`,
		name,
		ts.UTC().Format(time.RFC3339Nano),
		has,
	)
	if err != nil {
		t.Fatal(err)
	}
}

// TestDNSSECRatio ensures that the summary query in dnssecRatio
// correctly computes the fraction of checked domains that have DNSSEC.
func TestDNSSECRatio(t *testing.T) {
	db := testDB(t)
	names := seedDomains(t, db, 20)
	now := time.Now()
	for i, name := range names {
		has := i == 0
		insertCheck(t, db, name, now, has)
	}
	ratio, err := dnssecRatio(context.Background(), db, 20)
	if err != nil {
		t.Fatal(err)
	}
	if ratio < 4.9 || ratio > 5.1 {
		t.Fatalf("ratio %.1f not about 5", ratio)
	}
}

// TestListUnclassed verifies that listUnclassed only returns
// domains without a class assigned.
func TestListUnclassed(t *testing.T) {
	db := testDB(t)
	names := seedDomains(t, db, 3)
	_, err := db.Exec(
		"UPDATE domains SET class = 'Tech' WHERE name = ?",
		names[1],
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		"UPDATE domains SET class = '' WHERE name = ?",
		names[2],
	)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := listUnclassed(db, &buf); err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(buf.String()), "\n")
	want := []string{names[0], names[2]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v got %v", want, got)
	}
}

// TestNextDomainExcludesRecentlyChecked checks that nextDomain
// skips the most recently probed name when more than five exist.
func TestNextDomainExcludesRecentlyChecked(t *testing.T) {
	db := testDB(t)
	names := seedDomains(t, db, 6)
	now := time.Now()
	for i, name := range names[1:] {
		dur := time.Duration(6-i) * time.Hour
		insertCheck(t, db, name, now.Add(-dur), false)
	}
	recent := names[5]
	for i := 0; i < 20; i++ {
		_, name, err := nextDomain(context.Background(), db)
		if err != nil {
			t.Fatal(err)
		}
		if name == recent {
			t.Fatalf("returned recently checked %s", name)
		}
	}
}

// TestIndexQueryLatestCheck ensures that the domain listing query
// returns the newest DNSSEC check for each domain.
func TestIndexQueryLatestCheck(t *testing.T) {
	db := testDB(t)
	names := seedDomains(t, db, 3)
	now := time.Now()
	insertCheck(t, db, names[0], now.Add(-2*time.Hour), false)
	insertCheck(t, db, names[0], now.Add(-1*time.Hour), true)
	insertCheck(t, db, names[1], now.Add(-3*time.Hour), false)
	rows, err := db.Query(
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
		10,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ok bool
	for rows.Next() {
		var (
			rank    int
			name    string
			class   sql.NullString
			sec     sql.NullBool
			checked sql.NullTime
		)
		if err := rows.Scan(
			&rank,
			&name,
			&class,
			&sec,
			&checked,
		); err != nil {
			t.Fatal(err)
		}
		if name == names[0] {
			if !sec.Valid || !sec.Bool {
				t.Fatalf("latest check missing for %s", name)
			}
			ok = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("did not find %s", names[0])
	}
}
