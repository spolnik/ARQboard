# ARQboard

ARQboard is a simple, self-hostable kanban board with onboard wiki pages and an AI-native direction.

The current implementation plan is documented in [docs/INITIAL_SPEC.md](docs/INITIAL_SPEC.md).

## Current Skeleton

This repo now contains the Phase 0 service and web skeleton:

- Go CLI entrypoint: `arqboard serve`, `arqboard migrate`, and `arqboard admin create-user`.
- HTTP health checks: `/healthz` does not touch the database, `/readyz` checks PostgreSQL.
- PostgreSQL migrations through Goose-compatible SQL files.
- React + Vite + TypeScript + Tailwind frontend under `web/`.
- Dockerfile, Docker Compose Postgres service, Makefile, and CI workflow.

## Local Setup

For local development, ARQboard defaults to a file-backed SQLite database at `data/arqboard.db` when `DATABASE_URL` is not set. PostgreSQL remains the production target.

```bash
make migrate-up
cd web && npm install && npm test && npm run build
make dev
```

To test with PostgreSQL instead:

```bash
docker compose up -d postgres
$env:DATABASE_URL="postgres://arqboard:arqboard@localhost:5432/arqboard?sslmode=disable"
make migrate-up
make dev
```

Useful direct commands:

```bash
go test ./...
cd web && npm test
go run ./cmd/arqboard migrate
go run ./cmd/arqboard admin create-user --email admin@example.com --password "correct horse battery staple"
go run ./cmd/arqboard serve
```

Set `APP_ENV=production` in deployed environments. In production mode, `DATABASE_URL`, `APP_URL`, and `SESSION_SECRET` are required.
