# AGENTS.md

Guidance for Codex and other coding agents working in this repository.

## Product Direction

ARQboard is an open-source, self-hostable kanban board with onboard wiki pages and an AI-native direction. It should be easy for companies to run on their own infrastructure with a Docker image and PostgreSQL-compatible database such as AWS RDS.

Primary stack:

- Backend: Go.
- HTTP: `chi`.
- Database: PostgreSQL via `pgx`.
- Query generation: `sqlc`.
- Migrations: SQL migrations, preferably with `goose` unless the repository later chooses otherwise.
- Frontend: React + Vite + TypeScript + Tailwind CSS.
- Drag and drop: `dnd-kit`.
- Integration tests: Testcontainers.

## Engineering Values

- Prefer simplicity over cleverness.
- Keep code explicit, small, and easy to inspect.
- Choose boring, portable infrastructure over managed-service lock-in.
- Avoid framework magic when straightforward code is enough.
- Optimize for a self-hosted product that can run in AWS, on a VM, or in any Docker-capable environment.

## TDD And ATDD

Always write tests before implementation.

For user-visible features:

1. Start with an acceptance test that describes the externally observable behavior.
2. Add focused unit or integration tests for the lower-level behavior needed to make the acceptance test pass.
3. Implement the smallest amount of production code needed to make the tests pass.
4. Refactor only after the tests are green.

For bug fixes:

1. Add a failing regression test that reproduces the bug.
2. Fix the bug.
3. Keep the regression test.

If a test-first workflow is genuinely impractical for a small mechanical change, state why in the work summary and keep the change minimal.

## Database Rules

- All schema changes must be represented as committed SQL migration files.
- Do not rely on automatic schema synchronization in production.
- Keep migrations compatible with standard PostgreSQL and AWS RDS.
- Every application table must have a single `id` primary key backed by a UUID value. Join tables should still have their own UUID `id` primary key and use separate unique constraints for natural pairs such as `(workspace_id, user_id)`.
- Treat user-editable names, titles, and labels as display text only. Do not use them as durable identity. Use immutable UUID foreign keys, slugs where appropriate for URLs, or explicit internal keys for seeded/template records.
- Supabase may be used as a hosted PostgreSQL target for demos and testing, but application code must not depend on Supabase-specific APIs.
- Update `sqlc` queries and generated code whenever migrations or query contracts change.
- Prefer explicit SQL over ORM abstractions.

## Integration Tests

- Use Testcontainers for integration tests that require PostgreSQL.
- Do not require a developer to have a manually configured local Postgres instance for tests.
- Integration tests should run against the same SQL migrations used in production.
- Keep integration tests deterministic and isolated.
- Use clear test data builders or fixtures when they make tests easier to read.

## Performance And Blocking Behavior

- Keep request handlers fast and bounded.
- Use contexts, deadlines, and cancellation for database and external operations.
- Avoid unbounded goroutines, unbounded queues, and long blocking work in HTTP request paths.
- For work that may become slow, design an explicit async/job path rather than blocking the user request.
- Add indexes in migrations for new query patterns that need them.
- Keep frontend interactions responsive; optimistic UI is acceptable when backed by clear error handling.

## API And Error Handling

- Use JSON over HTTP under `/api`.
- Return consistent error shapes.
- Validate input at the boundary.
- Do not leak internal errors or secrets to clients.
- Keep authorization checks close to the operation they protect.

## Frontend Guidance

- Build the actual application surface, not a marketing landing page.
- Keep the UI dense, quiet, and work-focused.
- Use accessible controls and keyboard-friendly interaction patterns.
- Avoid decorative complexity.
- Treat the kanban board, card detail, and wiki editor as first-class product workflows.

## Operational Requirements

- The final app should support:
  - `arqboard serve`
  - `arqboard migrate`
  - `arqboard mcp`
  - `arqboard admin create-user`
- The Docker image should run without Docker Compose when provided the required environment variables.
- `/healthz` must not require database access.
- `/readyz` must verify database connectivity.
- Logs should be structured and suitable for CloudWatch or similar systems.
- The MCP server should use stdio for local trusted clients unless a remote authenticated transport is explicitly designed. MCP tools must call the same application stores and authorization boundaries as other product surfaces.

## Documentation

- Keep `docs/INITIAL_SPEC.md` aligned with major architecture decisions.
- Document new environment variables when they are introduced.
- Prefer short, accurate docs close to the feature over large speculative documents.
