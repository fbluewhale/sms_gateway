# Infrastructure layer

Infrastructure adapts external systems to application/domain contracts:

- `persistence`: PostgreSQL repositories and Goose migrations, including the
  transactional outbox and delivery inbox tables.
- `messaging`: RabbitMQ exchange/queue declaration, fair outbox publishing,
  publisher confirms, worker consumption, acknowledgements, deduplication, and
  refunds.
- `sms`: provider adapters plus Redis-coordinated round-robin routing and a
  shared closed/open/half-open circuit breaker for each provider. Lua scripts
  make circuit transitions atomic across worker replicas. The three mock
  providers have different latency and random failure profiles and are suitable
  only for local verification.
