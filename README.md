# dnssecmenot

I'm tired of telling people how to write a Bash `for` loop around `dig ds`. 

This project tracks DNSSEC adoption among the Tranco research list top 1000 domains.

## Development

1. Copy `.env.example` to `.env` and tweak settings as needed.
   The database file location can be changed via the `DB_PATH` variable.
2. Install frontend dependencies and build the CSS:

```bash
npm install
npm run build:css
```

3. Run the server:

```bash
go run .
```

