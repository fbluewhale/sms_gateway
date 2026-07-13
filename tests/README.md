# Tests

Tests are kept in this top-level directory so package code remains focused on
production behavior. The suite covers DTO mapping, configuration validation,
HTTP routing, SMS admission/deadline event creation, wallet arithmetic, and
concurrent wallet updates.

Run the full suite with:

```sh
go test ./...
go test -race ./...
```
