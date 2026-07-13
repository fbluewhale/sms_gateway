# Interface adapters

The HTTP adapter parses and validates JSON, calls application services, maps
results to response DTOs, and translates known errors to HTTP status codes.
Routing and authentication setup live in `http/router`; handlers should not
perform SQL, wallet arithmetic, or broker operations directly.
