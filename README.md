# Wallet Transfer Assignment Repository

This repository is a reusable coding assignment template for evaluating backend engineers on wallet transfers, idempotency, concurrency control, and double-entry ledger design.

## Included

- `ASSIGNMENT.md` - candidate-facing prompt
- `.github/pull_request_template.md` - required PR structure
- `.github/workflows/ci.yml` - lint, format, test placeholder workflow
- `.github/workflows/sonarqube.yml` - SonarQube pull request analysis
- `.github/copilot-instructions.md` - repository-level Copilot review guidance
- `evaluation_guide.md` - reviewer rubric
- `branch-protection-checklist.md` - GitHub setup checklist

## Intended use

1. Mark this repository as a GitHub template repository.
2. Create one private repository per candidate from the template.
3. Add the candidate as a collaborator.
4. Ask them to submit via a pull request into `main`.
5. Enable required checks, SonarQube, and Copilot review in GitHub.

## Notes

- Copilot automatic pull request review is configured in GitHub repository or organization settings, not purely through files in the repo.
- The `copilot-instructions.md` file included here provides repository-specific review guidance once Copilot review is enabled.
- The CI workflow is language-agnostic by default and expects you to set the `LINT_CMD`, `FORMAT_CHECK_CMD`, and `TEST_CMD` repository variables or replace the commands directly.

## How to Submit Assignment

1. **Fork this repository** to your own GitHub account.
2. Complete the assignment described in [`ASSIGNMENT.md`](./ASSIGNMENT.md).
3. **Raise a Pull Request** back to this repository (`main` branch) with your full solution.

Your PR branch should be named: `solution/<your-name>` (e.g., `solution/jane-doe`).

## Solution Scaffold

A starting project skeleton for the assignment lives in
[`solution/`](./solution), kept separate from the template docs above so
the two don't get tangled together. It implements the layered
architecture the brief asks for (handler -> service -> repository ->
domain), in Go with PostgreSQL, and is a self-contained Go module. It is
intentionally incomplete — most methods return `not implemented` — so a
candidate can fill in the actual transfer/idempotency/concurrency logic.

```text
solution/
  cmd/server/main.go             entry point: wiring and HTTP server startup
  internal/domain/               entities, state machine, sentinel errors
  internal/repository/           repository interfaces + Postgres implementations
  internal/service/              transfer workflow (idempotency, ledger, state transitions)
  internal/handler/http/         handlers, router, request/response DTOs, JSON response helpers
  internal/db/                   connection pooling (postgres.go), config (config.go)
  migrations/                    SQL migrations
  docs/design.md                 design notes (fill in before implementing, per the
                                  assignment's Documentation-First Workflow)
  docker-compose.yml             local PostgreSQL for development
  .env.example                   environment variables the server reads
  Makefile                       run / test / migrate shortcuts
```

Request/response DTOs live in `solution/internal/handler/http/dto.go`,
not in `internal/domain/` — they're HTTP wire shapes (JSON tags), and the
domain model shouldn't know about the transport layer. Shared
response-writing helpers live in
`solution/internal/handler/http/response.go`, scoped to the handler
package rather than a generic top-level `utils/`. Repositories depend on
a `DBTX` interface rather than a concrete `*pgxpool.Pool` (see
`solution/internal/repository/postgres/dbtx.go`), and the HTTP handler
depends on a `TransferCreator` interface rather than the concrete
service, so both layers are mockable in tests.

### How to Run

```bash
cd solution
cp .env.example .env
docker compose up -d          # starts PostgreSQL on localhost:5432
make migrate-up                # applies migrations/0001_init.up.sql
go run ./cmd/server
```

### How to Test

```bash
cd solution
go test ./...
```

### Where to Start

1. Fill in [`solution/docs/design.md`](./solution/docs/design.md) with
   the idempotency and concurrency strategy.
2. Implement the repository methods in
   `solution/internal/repository/postgres/`.
3. Implement `TransferService.CreateTransfer` in
   `solution/internal/service/`.
4. Add request validation in
   `solution/internal/handler/http/transfer_handler.go`.
5. Write tests (unit, idempotency replay, concurrency) alongside each
   package.
