# Infrastructure layer

Infrastructure adapts external systems to application/domain contracts:

- `persistence`: PostgreSQL repositories and Goose migrations, including the
  transactional outbox and delivery inbox tables.
- `messaging`: RabbitMQ exchange/queue declaration, fair outbox publishing,
  publisher confirms, worker consumption, acknowledgements, deduplication, and
  refunds.
- `sms`: provider adapters, round-robin routing, and an independent
  closed/open/half-open circuit breaker for each provider. The three mock
  providers have different latency and random failure profiles and are suitable
  only for local verification.
