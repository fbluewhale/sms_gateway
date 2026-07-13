# Domain layer

Domain packages contain business concepts and invariants:

- `wallet`: fixed-precision money, balance arithmetic, affordability, and
  wallet transactions.
- `channel`: named routing channels and active state.
- `sms`: line types, destination validation, and SMS cost concepts.

Domain code must not import Gin, GORM, RabbitMQ, or environment configuration.
