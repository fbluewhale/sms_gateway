# Command entry points

Each directory here builds one deployable process:

- `api-gateway`: HTTP API, database connection, migrations, and graceful HTTP
  shutdown.
- `outbox-dispatcher`: reads pending PostgreSQL outbox rows and publishes
  confirmed persistent messages to RabbitMQ.
- `sms-worker`: consumes exactly one line (`express` or `normal`) with a
  configurable prefetch and provider-call concurrency.

Commands load shared environment configuration from `internal/config` and
should remain thin composition roots. Business rules belong in `internal`.
