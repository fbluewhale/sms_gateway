# SMS Gateway

HTTP service for charging wallets and dispatching SMS requests through named channels.

See [docs/architecture.md](docs/architecture.md) for package boundaries,
failure semantics, SLA behavior, and editable Draw.io diagrams. Each major
source tree also has a local README describing its responsibility.

## Docker Compose

Start the API and PostgreSQL together:

```sh
docker compose -f deployments/docker/compose.yaml up --build
```

The API listens on `http://localhost:8080`. The local admin API key is
`local-admin-key`. Override local credentials and the published port through
environment variables when needed:

```sh
ADMIN_API_KEY=change-me POSTGRES_PASSWORD=change-me APP_PORT=18080 \
  docker compose -f deployments/docker/compose.yaml up --build
```

Docker build traffic uses the host proxy at `http://127.0.0.1:10808` by default.
The running API reaches the same proxy through Docker's host gateway. Override
`HTTP_PROXY`, `HTTPS_PROXY`, or `NO_PROXY` when needed.

Stop the stack with `docker compose -f deployments/docker/compose.yaml down`. Add
`--volumes` only when you also want to delete the PostgreSQL data volume.

The stack also runs RabbitMQ, one transactional-outbox dispatcher, three
express workers, and two normal workers. Scale the independently routed worker
pools without changing the API:

```sh
EXPRESS_WORKER_REPLICAS=6 NORMAL_WORKER_REPLICAS=2 \
  docker compose -f deployments/docker/compose.yaml up --build -d
```

Each line has a dedicated durable quorum queue and a dedicated worker pool, so
a 10,000 req/s burst on one line cannot consume the other line's consumers.
The outbox dispatcher also takes a bounded batch from each line on every pass;
this prevents starvation before events reach RabbitMQ. Worker concurrency and
replica counts are independent per line. RabbitMQ management is available at
`http://localhost:15672`.

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
| `BROKER_URL` | `amqp://guest:guest@localhost:5672/` |
| `EXPRESS_SMS_SLA` | `5s` |
| `EXPRESS_INFLIGHT_LIMIT` | `100` per API replica |
| `NORMAL_INFLIGHT_LIMIT` | `20` per API replica |

Set `APP_ENV=production` in every production deployment. Production startup
fails unless `DB_PASSWORD` and `ADMIN_API_KEY` are explicitly configured.

## Routes

- `POST /api/v1/sms` charges the channel wallet, queues the SMS, and returns
  `202 Accepted`. Express and normal messages are processed by independent
  workers so one line cannot block the other. If asynchronous delivery fails,
  the charged amount is credited back to the wallet and recorded as a refund
  transaction using the SMS message ID.
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

## Delivery guarantees

The API commits the wallet debit, ledger entry, and outbox event in one
PostgreSQL transaction. The dispatcher uses persistent messages, durable quorum
queues, mandatory routing, and publisher confirms. Consumers use manual
acknowledgements and a `sms_deliveries` inbox table keyed by message ID, making
duplicate broker deliveries safe across multiple consumer replicas.

Strict exactly-once delivery across PostgreSQL, RabbitMQ, and an external SMS
provider requires provider cooperation. A production sender must pass
`message_id` as the provider's idempotency key. With that contract, retries
produce one provider-side effect; without it, the infrastructure guarantees
at-least-once attempts with application-side deduplication. Failed deliveries
are refunded atomically with their terminal delivery record.

Express events carry an absolute deadline calculated from `EXPRESS_SMS_SLA`.
Workers never start a provider attempt after that deadline; an expired request
is terminally marked and refunded atomically. This is an enforceable processing
deadline, not a promise that an external provider will deliver to a handset in
that time. Production capacity must keep accepted Express throughput below
`replicas * concurrency / provider_latency`, with headroom for failures. Monitor
the age of the oldest `sms.express` message and scale the Express pool before it
approaches the configured deadline.

Ingress capacity is isolated too. Each API replica reserves independent
in-flight budgets for Express and Normal requests. When one line exhausts its
budget, only that line receives `429 Too Many Requests` with `Retry-After: 1`;
the other line's request slots remain available. Tune these limits below the
database pool capacity and retry rejected requests with jitter.
