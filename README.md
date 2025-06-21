# dnssecmenot

This project tracks DNSSEC adoption among the top domains. It is currently under heavy development.

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

Then visit `http://localhost:8080/lookup/example.com` to perform a test DNS lookup.
