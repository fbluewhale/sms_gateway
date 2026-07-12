# SMS Gateway

HTTP service for charging wallets and dispatching SMS requests through named channels.

## Docker Compose

Start the API and PostgreSQL together:

```sh
docker compose -f deployment/compose.yaml up --build
```

The API listens on `http://localhost:8080`. The local admin API key is
`local-admin-key`. Override local credentials and the published port through
environment variables when needed:

```sh
ADMIN_API_KEY=change-me POSTGRES_PASSWORD=change-me APP_PORT=18080 \
  docker compose -f deployment/compose.yaml up --build
```

Docker build traffic uses the host proxy at `http://127.0.0.1:10808` by default.
The running API reaches the same proxy through Docker's host gateway. Override
`HTTP_PROXY`, `HTTPS_PROXY`, or `NO_PROXY` when needed.

Stop the stack with `docker compose -f deployment/compose.yaml down`. Add
`--volumes` only when you also want to delete the PostgreSQL data volume.

## Configuration

The service is configured through environment variables. Local development uses
the PostgreSQL defaults shown below, so `go run ./cmd/api-gateway` works against
a local `sms_gateway` database. Administrative endpoints require the API key in
the `X-Admin-API-Key` header.

| Variable | Default |
|---|---|
| `APP_ENV` | `local` |
| `SERVER_PORT` | `8080` |
| `DB_HOST` | `localhost` |
| `DB_PORT` | `5432` |
| `DB_USER` | `postgres` |
| `DB_NAME` | `sms_gateway` |
| `DB_SSLMODE` | `disable` locally, `require` in production |
| `DB_PASSWORD` | `postgres` locally, required in production |
| `ADMIN_API_KEY` | `local-admin-key` locally, required in production |

Set `APP_ENV=production` in every production deployment. Production startup
fails unless `DB_PASSWORD` and `ADMIN_API_KEY` are explicitly configured.

## Routes

- `POST /api/v1/sms` sends an SMS and charges its channel wallet.
- Wallet, transaction, and channel routes under `/api/v1` are administrative
  and require `X-Admin-API-Key`.

## Verification

```sh
go test -race ./...
go vet ./...
go build ./...
```

Wallet balances are represented internally as fixed four-decimal units. Credit
and debit ledger records are committed atomically while locking the wallet row,
preventing concurrent balance overwrites.
