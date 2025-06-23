# dnssecme-not Project Plan

This document outlines the comprehensive plan for **dnssecme-not**, a Go-based service that tracks DNSSEC adoption among the top domains, backed by SQLite and rendered via a simple Tailwind CSS frontend with htmx for interactivity.

## Table of Contents
1. [Project Overview](#project-overview)
2. [Architecture](#architecture)
3. [Libraries & Tools](#libraries--tools)
4. [Task Checklist](#task-checklist)
5. [Continuous DNS Checking](#continuous-dns-checking)
6. [Rate Limiting Strategy](#rate-limiting-strategy)
7. [Configuration](#configuration)
8. [CI/CD & Deployment](#ci-cd--deployment)

## Project Overview

**dnssecme-not** periodically evaluates whether the top domains (per Tranco ranking) have DNSSEC enabled by querying for DS records. It stores results in a local SQLite database and exposes a minimal web UI styled with Tailwind CSS.

## Architecture

```text
                 +------------------+
                 |  Tranco list CSV |
                 +---------+--------+
                           |
                           v
                  +-----------------+
                  |   Updater Job   |  (daily/hourly)
                  +---+-------------+
                      | inserts/updates
                      v
  +------------+   +-----------------+    +--------------+
  |  Scheduler |-->| DNS Checker     |--->|  SQLite DB   |
  | (ticker)   |   | (miekg/dns + RL)|    +------+-------+
  +------------+   +-----------------+           |
                                                   v
                                            +--------------+
                                           | HTTP Server  |
                                           | (net/http)   |
                                            +------+-------+
                                                   |
                                                   v
                                           +---------------+
                                           | Tailwind CSS  |
                                           | Frontend (no JS) |
                                           +---------------+
```

## Libraries & Tools

- **Go Modules** (`go.mod`) for dependency management
- **DNS lookups**: `github.com/miekg/dns`
- **SQLite driver**: `github.com/mattn/go-sqlite3`
 - **HTTP routing**: built-in `net/http` mux
 - **Rate limiter**: `golang.org/x/time/rate`
- **Tailwind CSS** for styling (via `tailwindcss` CLI or embedded CDN)
- **go:embed** (builtin) for bundling static assets
- **go-dotenv** (`github.com/joho/godotenv`) for `.env` config
- **htmx** via CDN for minimal dynamic behaviour

## Task Checklist

### Initialization
- [x] Initialize Go module (`go mod init`)
- [x] Add `.gitignore`, `.env.example`, and basic folder layout

### Data Ingestion
- [x] Download/parse Tranco top domains CSV
- [x] Upsert domain list into SQLite (`domains` table)
- [x] Figure out how to do this as a migration/fixture process, rather than on every boot.
- [x] Take the SQLite database path from an env var.
- [x] Seed domain classifications from `classes.json` for any matches
- [x] Store a `class` field in the `domains` table
- [x] Provide a CLI flag that loads classes from a specified JSON file
- [x] Provide a CLI flag that lists all unclassed domains
- [ ] Add index on `domains.class` for faster queries
 
### DNS Checking
- [x] Design DB schema: `domains`, `dns_checks` tables
 - [ ] Implement rate-limited DS record lookup (using `miekg/dns` + `rate.Limiter`)
 - [x] Randomly select from names in the top list to re-check based on when the last check was.
 - [x] Write background scheduler (ticker) for periodic checks
- [ ] Handle failures & retries (backoff, logging)
- [ ] Add a command-line one-time check that updates the whole list interactively.

### Web Server & Frontend
- [x] Implement HTTP server using `net/http`
 - [x] Define route for listing domains and their latest DNSSEC status
 - [ ] Add detail view and filtering
- [x] Integrate Tailwind CSS workflow (build or CDN)
- [x] Build minimal HTML templates (htmx only)
- [ ] Come up with a qualitative color scheme (expressed in standard tailwind colors) for classes
- [ ] Show the classification next to each domain in the list
- [ ] Add filtering by classification on the index view
- [ ] Display a simple chart summarizing counts per class

### Configuration & Env
- [x] Add `.env` support for settings (server address)
- [x] Create and document sane defaults in the code.

### Tests & Quality
- [ ] Unit tests for any actual logic we write (but don't mock DNS or the network)
- [ ] Linting (`golangci-lint`)

### CI/CD & Deployment
 - [x] Dockerfile (slim image) for service
 - [ ] Deployment docs / `Makefile`

## Continuous DNS Checking

A ticker based scheduler kicks off a DNSSEC check job at a configurable interval. The job:
1. Reads the list of tracked domains from the DB.
2. Performs the DS record lookup sequentially.
3. Persists timestamped results in `dns_checks`.

Failed lookups (timeouts, network errors) are retried once with exponential backoff. If a domain consistently fails 3 times, it is marked for manual review.

## Rate Limiting Strategy

To avoid overloading upstream DNS servers or triggering rate-based blocks:
- Use a **token bucket** (`rate.NewLimiter`) with configurable rate (e.g., 100 requests/min).
- Limit concurrent workers (e.g., max 10 goroutines).
- Respect DNS response codes and backoff on SERVFAIL / REFUSED.
- Make rate limit parameters adjustable via environment variables.

```env
# .env.example
CHECK_INTERVAL=1m
DB_PATH=./dnssec.db
DNS_RATE=100/m
DNS_BURST=10
CONCURRENT_WORKERS=10
```


## Frontend Design Plan

The index view uses Tailwind CSS from the CDN. A grid with four columns shows
rank, domain, DNSSEC status and the last check time. About fifty rows render per
page with `page` as a query parameter. Status text is colored green or red. The
layout is responsive and uses htmx for infinite scroll.

## Deprecated Tasks

- Replaced `gocron` with a simple time.Ticker based scheduler.
- `SCHEDULER_INTERVAL` env var superseded by `CHECK_INTERVAL`.
- The worker pool idea was dropped in favor of sequential checks.
