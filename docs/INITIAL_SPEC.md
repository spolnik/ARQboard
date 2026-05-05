# ARQboard Initial Product And Skeleton Spec

## Purpose

ARQboard is an open-source, self-hostable kanban workspace for teams that want a lightweight Jira-style board with built-in project knowledge pages and room for AI-native workflows later.

The first version should be boring to operate: one Docker image, one Postgres database, one HTTP service, and no required SaaS dependencies.

## Goals

- Provide a simple kanban board with boards, columns, cards, comments, and activity history.
- Support company self-hosting on AWS or similar infrastructure.
- Run against standard PostgreSQL, including AWS RDS.
- Ship as a single Docker image that serves both the API and the web UI.
- Keep the backend explicit, inspectable, and easy for Codex-driven development.
- Leave clear extension points for wiki pages, attachments, SSO, and AI features.

## Non-Goals For V1

- Full Jira parity.
- Complex custom workflows.
- Estimates, burndown charts, or reporting dashboards.
- Marketplace-style plugins.
- Multi-region high availability.
- Realtime collaboration as a hard requirement.
- Hard dependency on Vercel, Supabase, Clerk, Firebase, PocketBase, or other managed services.

## Stack Decision

### Backend

- Language: Go.
- HTTP routing: `chi`.
- Database: PostgreSQL.
- Postgres driver: `pgx`.
- Query layer: `sqlc`.
- Migrations: `goose` or `golang-migrate`; default recommendation is `goose` for simple SQL migration files and easy embedding.
- Data identity: every application table has a single UUID `id` primary key. Human-editable names and titles are display text, not durable identity.
- Auth: first-party session auth stored in Postgres for V1, with OIDC support planned.
- Config: environment variables.
- Logging: structured JSON logs.
- Health checks: `/healthz` and `/readyz`.

### Frontend

- React.
- Vite.
- TypeScript.
- Tailwind CSS.
- shadcn/ui-style components.
- `dnd-kit` for card drag and drop.
- TanStack Query for API cache and mutations.
- React Router if more than one major client-side route is needed.

### Runtime And Packaging

- Single Docker image.
- Single long-running Go process.
- Go server exposes API routes and serves the built Vite frontend.
- Static frontend assets are embedded into the Go binary or copied into the image and served by Go.
- Required external dependency: PostgreSQL.
- Optional later dependency: S3-compatible object storage for attachments.

## Deployment Shape

Reference self-hosted command shape:

```bash
arqboard serve
arqboard migrate
arqboard mcp
arqboard admin create-user
```

`arqboard serve` applies embedded migrations before opening the HTTP listener, and it should no-op cleanly when the target database is already current. `arqboard migrate` remains available for explicit CI, release, or operator-controlled migration runs.

`arqboard mcp` runs a local stdio Model Context Protocol server for trusted local clients. It exposes board, card search, card update, card movement, and wiki tools backed by the same database stores as the HTTP API.

Reference Docker usage:

```bash
docker run --rm --env-file .env ghcr.io/spolnik/arqboard:latest migrate

docker run -p 8080:8080 \
  --env-file .env \
  ghcr.io/spolnik/arqboard:latest serve
```

Reference AWS deployment:

- ECS/Fargate or EKS runs the ARQboard container.
- ALB routes HTTP traffic to the container.
- RDS PostgreSQL stores application data.
- AWS Secrets Manager or SSM Parameter Store provides secrets.
- CloudWatch receives logs.
- S3 stores attachments later.

## Required Environment Variables

Initial required variables:

```txt
DATABASE_URL=postgres://user:password@host:5432/arqboard?sslmode=require
APP_URL=https://arqboard.example.com
SESSION_SECRET=replace-with-random-secret
HTTP_ADDR=:8080
```

For local development only, missing `DATABASE_URL` falls back to a SQLite database at `sqlite://data/arqboard.db`. Deployed production environments should set `APP_ENV=production` and provide PostgreSQL configuration explicitly.

Planned optional variables:

```txt
OIDC_ISSUER_URL=
OIDC_CLIENT_ID=
OIDC_CLIENT_SECRET=
S3_BUCKET=
S3_REGION=
S3_ENDPOINT=
S3_ACCESS_KEY_ID=
S3_SECRET_ACCESS_KEY=
```

## Initial Repository Skeleton

Target layout:

```txt
.
├── cmd/
│   └── arqboard/
│       └── main.go
├── internal/
│   ├── app/
│   ├── auth/
│   ├── board/
│   ├── card/
│   ├── config/
│   ├── db/
│   ├── http/
│   └── wiki/
├── migrations/
├── queries/
├── web/
│   ├── src/
│   ├── index.html
│   ├── package.json
│   └── vite.config.ts
├── docs/
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── sqlc.yaml
├── Makefile
└── README.md
```

## Initial Domain Model

V1 tables should start small:

```txt
users
sessions
workspaces
workspace_members
boards
board_members
columns
cards
sprints
epics
card_dependencies
card_comments
activity_events
wiki_pages
labels
card_labels
```

Planned later tables:

```txt
attachments
integrations
api_tokens
automation_rules
```

## V1 Product Surface

### Authentication

- Email/password login.
- Secure HTTP-only session cookies.
- Passwords hashed with a modern password hashing algorithm.
- Initial admin user can be created by CLI.
- OIDC login should be designed as a future addition, not required for the first skeleton.

### Workspaces, Teams, And Boards

- A workspace contains members and teams.
- Each team owns exactly one board. Creating a team provisions its team board automatically, and additional boards for the same team are rejected.
- Board names, team names, and column names are display text only. Team, board, sprint, and column identity is held by UUIDs and stable system keys where needed.
- A board contains ordered columns and cards. Default workflow columns have fixed product meaning; adding a column is admin functionality, while renaming workflow states is intentionally not exposed in the application UI.
- Sprints belong to one team-owned board and are named from their ISO week, for example `Sprint 2026-W19`.
- Creating a sprint from the current week auto-starts it when the team has no active sprint. Completing a sprint moves selected cards into a planned next sprint and returns the remaining cards to backlog.
- V1 can assume one workspace per deployment if that simplifies the first implementation, but the schema should not block multiple workspaces later.

### Cards

Each card should support:

- Title.
- Description.
- Status via column placement.
- Assignee as a named workspace member. User display names are required separately from email addresses and are the primary assignee label in card boxes.
- Priority.
- Labels.
- Sort position.
- Due date.
- Created/updated timestamps.

### Comments And Activity

- Cards have comments.
- Important changes create activity events.
- Activity events should be append-only.

### Roadmap

- Teams can group cards into epics.
- Epics have UUID identity, display title, slug, description, status, start date, and target date.
- Cards can have blocking dependencies on other cards in the same team.
- Roadmap views should show epic progress, blocked work, and cards not yet assigned to an epic.

### Wiki Pages

Wiki pages are first-class project knowledge pages linked to a workspace or board.

V1 wiki scope:

- Title.
- Slug.
- Markdown body.
- Workspace relation.
- Optional board relation.
- Created/updated timestamps.

### AI-Native Direction

The skeleton should avoid hard-coding any AI provider. Future AI features should be implemented behind interfaces.

Possible later features:

- Summarize card activity.
- Draft card descriptions from notes.
- Turn wiki page sections into cards.
- Natural language board search.
- Triage suggestions.

## Initial API Shape

Use JSON over HTTP under `/api`.

Current local baseline implements first-party login/logout for CLI-created admin users, protected workspace API routes, and the default-board slice with persisted cards, member assignees, labels, board filters, movement, sprint planning, roadmap epics, card dependencies, card detail updates, comments, activity events, and wiki page list/get/create/update endpoints. The same store path supports SQLite for zero-configuration local development and PostgreSQL for production-style deployments.

Suggested endpoints:

```txt
GET    /api/me
POST   /api/auth/login
POST   /api/auth/logout

GET    /api/workspaces
POST   /api/workspaces
GET    /api/workspaces/{workspaceID}

GET    /api/boards
POST   /api/boards
GET    /api/boards/{boardID}

POST   /api/boards/{boardID}/columns
PATCH  /api/columns/{columnID}
DELETE /api/columns/{columnID}

POST   /api/cards
GET    /api/cards/{cardID}
PATCH  /api/cards/{cardID}
DELETE /api/cards/{cardID}
POST   /api/cards/{cardID}/move
PATCH  /api/cards/{cardID}/sprint

GET    /api/cards/{cardID}/comments
POST   /api/cards/{cardID}/comments

GET    /api/planning?boardId={boardID}
POST   /api/sprints
POST   /api/sprints/{sprintID}/start
POST   /api/sprints/{sprintID}/complete

GET    /api/roadmap?teamId={teamID}
POST   /api/epics
PATCH  /api/epics/{epicID}
PATCH  /api/cards/{cardID}/epic
POST   /api/cards/{cardID}/dependencies
DELETE /api/card-dependencies/{dependencyID}

GET    /api/wiki
POST   /api/wiki
GET    /api/wiki/{pageID}
PATCH  /api/wiki/{pageID}
DELETE /api/wiki/{pageID}

GET    /healthz
GET    /readyz
```

`POST /api/sprints` requires `boardId`. `POST /api/sprints/{sprintID}/complete` accepts `rollover` decisions with `cardId` and an optional planned `sprintId`; cards omitted from rollover or sent with an empty `sprintId` return to backlog.

## Frontend Screens

Initial screens:

- Login.
- Workspace/board shell.
- Kanban board.
- Board filter toolbar for assignee, label, priority, due status, and text search.
- Roadmap dashboard with epics, unassigned cards, and dependency mapping.
- Planning dashboard.
- Card detail drawer or modal.
- Wiki page list.
- Wiki page editor.
- Basic settings/admin page.

Design direction:

- Quiet, dense, work-focused UI.
- Fast keyboard/mouse workflows.
- No marketing landing page inside the app.
- Use compact controls and predictable navigation.

## Build And Development Commands

Target commands:

```bash
make dev
make test
make lint
make generate
make migrate-up
make docker-build
```

Local development should use Docker Compose for Postgres:

```bash
docker compose up -d postgres
make migrate-up
make dev
```

The final container should run without Docker Compose as long as it has `DATABASE_URL`.

## Skeleton Acceptance Criteria

The first skeleton is complete when:

- `go test ./...` runs.
- `npm test` or equivalent frontend check runs, if frontend tests exist.
- `docker build .` succeeds.
- `docker run ... serve` applies migrations and starts one HTTP server.
- `docker run ... migrate` applies migrations to Postgres.
- `/healthz` returns OK without database access.
- `/readyz` verifies database connectivity.
- The root URL serves the React app.
- The app can create an admin user through CLI or seed command.

## Implementation Phases

### Phase 0: Repository Skeleton

- Add Go module.
- Add Vite React frontend.
- Add Dockerfile and docker-compose.
- Add config loader.
- Add HTTP server with health endpoints.
- Add migration command.
- Add SQL migration baseline.
- Add CI workflow.

### Phase 1: Auth And Boards

- Add users and sessions.
- Add login/logout.
- Add workspace, board, column, and card CRUD.
- Add card movement endpoint.
- Build usable kanban UI.

### Phase 2: Collaboration Basics

- Add comments.
- Add activity events.
- Add board members.
- Add basic authorization checks.

### Phase 3: Wiki And Attachments

- Add wiki pages.
- Add Markdown editor.
- Add S3-compatible attachment storage.

### Phase 4: Enterprise Self-Hosting

- Add OIDC.
- Add backup/restore documentation.
- Add Helm chart or ECS task definition examples.
- Add metrics endpoint.
- Add audit/event export.

## Open Decisions

- Migrations: use Goose-compatible SQL migrations.
- Frontend assets: copy the Vite build into the Docker image and serve it from `WEB_DIST_DIR`.
- Use generated `sqlc` code for application query paths, while allowing small handwritten bootstrap queries for CLI setup commands.
- Local development can use SQLite as a zero-configuration file-backed database; PostgreSQL remains the production database and generated-query target.
- Session storage uses database-backed hashed opaque tokens; choose CSRF strategy.
- Decide whether V1 supports multiple workspaces in UI or only in schema.
