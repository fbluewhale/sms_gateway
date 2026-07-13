# SMS Gateway Architecture

## Boundaries

The project follows a dependency direction from transport to application to
domain, with infrastructure implementations kept behind application/domain
interfaces:

```text
HTTP router/handler -> application services -> domain rules
                                  |
                                  +-> persistence repositories
                                  +-> transactional outbox

outbox dispatcher -> RabbitMQ exchange -> dedicated Express/Normal queues
                                              |
                                              +-> Express worker pool -> SMS provider
                                              +-> Normal worker pool -> SMS provider
```

The API process accepts and charges an SMS request in one PostgreSQL
transaction. That transaction writes the wallet debit, ledger row, and outbox
event. The dispatcher publishes the event with publisher confirms. Workers use
manual acknowledgements and `sms_deliveries.message_id` as an inbox/deduplication
key.

## Channel isolation and SLA

Express and Normal use separate RabbitMQ queues, worker deployments, prefetch
windows, and concurrency limits. The dispatcher publishes a bounded batch for
each routing key on every pass, so a hot channel cannot hide a low-volume
channel in the outbox. API replicas also have independent in-flight admission
budgets; an exhausted channel returns HTTP 429 while the other channel remains
available.

Express events carry `deadline_at`. A worker will not start an external provider
attempt after that deadline and refunds the charge atomically. This is a
processing deadline for accepted requests. End-to-end handset delivery still
depends on provider availability and provider-side idempotency.

## Failure and retry behavior

- A database transaction failure means no charge and no outbox event commit.
- An unconfirmed publish leaves the outbox row pending for a later retry.
- A worker failure negatively acknowledges and requeues the message.
- A successful or refunded message is terminal in `sms_deliveries`; duplicate
  broker deliveries do not repeat the business effect.
- Provider failures refund the original SMS cost in the same database
  transaction as the terminal delivery record.

## Diagrams

- [Component architecture](../deployments/docker/architecture.drawio)
- [SMS request and delivery sequence](../deployments/docker/sms-delivery-sequence.drawio)

These files are editable in diagrams.net/Draw.io and intentionally use only
standard shapes and connectors so they can be imported without plugins.
