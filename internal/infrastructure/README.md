# Infrastructure layer

Infrastructure adapts external systems to application/domain contracts:

- `persistence`: PostgreSQL repositories and Goose migrations, including the
  transactional outbox and delivery inbox tables.
- `messaging`: RabbitMQ exchange/queue declaration, fair outbox publishing,
  publisher confirms, worker consumption, acknowledgements, deduplication, and
  refunds.
- `sms`: provider adapters; the current mock sender is suitable only for local
  verification.
