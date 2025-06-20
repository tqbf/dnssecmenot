# dnssecmenot

This project tracks DNSSEC adoption among the top domains. It is currently under heavy development.

## Development

1. Copy `.env.example` to `.env` and tweak settings as needed.
2. Run the server:

```bash
go run .
```

Then visit `http://localhost:8080/lookup/example.com` to perform a test DNS lookup.
