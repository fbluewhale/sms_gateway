# Application layer

Application services coordinate use cases without owning HTTP, SQL, or broker
protocol details. SMS execution validates input, resolves the channel and
price, applies per-line admission capacity, and asks the wallet repository to
atomically debit and enqueue an outbox event.

DTOs in the subpackages translate request/response shapes at the boundary.
