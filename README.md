# ARQboard

ARQboard is a simple, self-hostable kanban board with onboard wiki pages and an AI-native direction.

The current implementation plan is documented in [docs/INITIAL_SPEC.md](docs/INITIAL_SPEC.md).

## Current App

This repo now contains the Go service, database-backed board API, and React web app:

- Go CLI entrypoint: `arqboard serve`, `arqboard migrate`, `arqboard mcp`, and `arqboard admin create-user`.
- HTTP health checks: `/healthz` does not touch the database, `/readyz` checks database connectivity.
- PostgreSQL and SQLite migrations through Goose-compatible SQL files.
- `serve` applies embedded migrations before it starts accepting traffic; `migrate` remains available as an explicit pre-flight command.
- CLI-created admin users can sign in through DB-backed HTTP-only sessions.
- A seeded default workspace board with persisted columns, cards, card creation, and card movement.
- Structured card metadata with named workspace-member assignees, labels, and board filters for focused views.
- Sprint planning with named board-scoped sprints, backlog assignment, one active sprint per board, and explicit close-sprint rollover choices.
- Authenticated JSON API endpoints for boards, card creation, card moves, sprint planning, card detail, comments, and wiki pages.
- Local stdio MCP server for board, card, and wiki planning tools.
- React + Vite + TypeScript + Tailwind frontend under `web/`.
- Drag-and-drop card movement through `dnd-kit`, using the whole card as the drag surface.
- Dockerfile, Docker Compose Postgres service, Makefile, and CI workflow.

## Local Setup

For local development, ARQboard defaults to a file-backed SQLite database at `data/arqboard.db` when `DATABASE_URL` is not set. PostgreSQL remains the production target.

```bash
make migrate-up
go run ./cmd/arqboard admin create-user --email admin@example.com --password "correct horse battery staple" --name "Admin"
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
go run ./cmd/arqboard admin create-user --email admin@example.com --password "correct horse battery staple" --name "Admin"
go run ./cmd/arqboard serve
go run ./cmd/arqboard mcp
```

`arqboard mcp` runs a local stdio MCP server for trusted local clients. It applies migrations, opens the configured database, and exposes tools for listing boards, searching cards, creating boards/cards/wiki pages, updating cards/wiki pages, and moving cards.

Set `APP_ENV=production` in deployed environments. In production mode, `DATABASE_URL`, `APP_URL`, and `SESSION_SECRET` are required.
