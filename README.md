# go-users-api

A small REST API built with Go, [Chi](https://github.com/go-chi/chi) and PostgreSQL,
structured with a clean, layered architecture.

## Architecture

```
cmd/api/                 Application entrypoint (wiring + graceful shutdown)
internal/
  config/                Environment configuration
  domain/                Entities, input DTOs and sentinel errors (no dependencies)
  repository/            Persistence interface
    postgres/            pgx implementation of the repository
  service/               Business rules and validation
  handler/               HTTP handlers, request/response DTOs, routing
  httpx/                 Shared HTTP helpers (JSON, errors, decoding)
migrations/              SQL schema
```

Dependencies point inward: `handler -> service -> repository interface -> postgres`.
The domain layer has no knowledge of HTTP or the database.

## Endpoints

| Method | Path                 | Description        |
|--------|----------------------|--------------------|
| GET    | `/healthz`           | Health check       |
| POST   | `/api/v1/users`      | Create a user      |
| GET    | `/api/v1/users`      | List users         |
| GET    | `/api/v1/users/{id}` | Get a user by id   |
| PUT    | `/api/v1/users/{id}` | Update a user      |
| DELETE | `/api/v1/users/{id}` | Delete a user      |

List supports `?limit=` (default 20, max 100) and `?offset=`.

### Create

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Ada Lovelace","email":"ada@example.com"}'
```

```json
{
  "id": "b6f1...",
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "created_at": "2026-11-09T12:00:00Z",
  "updated_at": "2026-11-09T12:00:00Z"
}
```

## Running

1. Start PostgreSQL (Docker) and apply the schema automatically on first boot:

   ```bash
   docker compose up -d
   ```

   If you use an existing Postgres instance instead, run the migration:

   ```bash
   psql "$DATABASE_URL" -f migrations/0001_init.sql
   ```

2. Create your env file and start the API:

   ```bash
   cp .env.example .env
   go run ./cmd/api
   ```

   On PowerShell without a `.env` file:

   ```powershell
   $env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/users?sslmode=disable"
   go run ./cmd/api
   ```

3. Check it is up:

   ```bash
   curl http://localhost:8080/healthz
   ```

## Testing in VS Code

Install the **REST Client** extension by Huachao Mao. With the API running, open
`api.http` and use the **Send Request** link above each request. The file includes
health check and user create/list/get/update/delete requests; the user ID is read
from the create response for the later requests.

The API can also be documented with **OpenAPI (Swagger)**. OpenAPI describes the
endpoints in a YAML or JSON file, and Swagger UI can display that specification
and let you send requests from a browser. This project serves the specification at
`http://localhost:8080/openapi.yaml` and Swagger UI at `http://localhost:8080/swagger`.
Swagger UI loads its assets from unpkg, so viewing the UI requires an internet connection.

## Common commands

```bash
make run     # run the API
make build   # build to bin/api
make test    # run tests
make vet     # go vet
make migrate # apply SQL migration
```
