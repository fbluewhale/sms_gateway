# Internal packages

`internal` contains implementation details that are not part of a reusable
external Go API. The dependency flow is:

`interfaces/http` -> `application` -> `domain`, while `infrastructure` provides
database, broker, and provider implementations used by the command entry
points.

Packages should expose interfaces at the consumer boundary and keep transport,
SQL, and RabbitMQ details out of domain entities.
