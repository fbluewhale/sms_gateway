# Graph Report - code_challenge  (2026-07-14)

## Corpus Check
- 53 files · ~17,064 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 395 nodes · 577 edges · 36 communities (23 shown, 13 thin omitted)
- Extraction: 80% EXTRACTED · 20% INFERRED · 0% AMBIGUOUS · INFERRED: 118 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `93674f8b`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 30|Community 30]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 33|Community 33]]
- [[_COMMUNITY_Community 34|Community 34]]
- [[_COMMUNITY_Community 35|Community 35]]

## God Nodes (most connected - your core abstractions)
1. `run()` - 17 edges
2. `MustMoney()` - 14 edges
3. `Worker` - 11 edges
4. `Load()` - 10 edges
5. `NewRoundRobinSender()` - 10 edges
6. `SMSHandler` - 10 edges
7. `writeServiceError()` - 10 edges
8. `NewService()` - 10 edges
9. `discardLogger()` - 10 edges
10. `Money` - 9 edges

## Surprising Connections (you probably didn't know these)
- `TestMoneySubtract()` --calls--> `MustMoney()`  [INFERRED]
  tests/wallet_entity_test.go → internal/domain/wallet/entity.go
- `run()` --calls--> `NewPostgresChannelRepository()`  [INFERRED]
  cmd/api-gateway/main.go → internal/infrastructure/persistence/channel_repository.go
- `run()` --calls--> `NewPostgresWalletRepositoryWithCache()`  [INFERRED]
  cmd/api-gateway/main.go → internal/infrastructure/persistence/wallet_repository.go
- `run()` --calls--> `NewPostgresSMSCostRepository()`  [INFERRED]
  cmd/api-gateway/main.go → internal/infrastructure/persistence/sms_cost_repository.go
- `run()` --calls--> `NewSMSDeliveryRepository()`  [INFERRED]
  cmd/api-gateway/main.go → internal/infrastructure/persistence/delivery_repository.go

## Communities (36 total, 13 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.08
Nodes (28): channelStub, costStub, senderStub, NewService(), NewServiceWithExpressSLA(), NewServiceWithPolicy(), NewServiceWithReservation(), TestExecuteChargesAndSends() (+20 more)

### Community 1 - "Community 1"
Cohesion: 0.06
Nodes (12): AdminService, NewAdminService(), SMSDeliveryRepository, MockProvider, TestMoneySubtract(), TestNewMoney(), NewMoney(), TestMoneySubtract() (+4 more)

### Community 2 - "Community 2"
Cohesion: 0.13
Nodes (26): circuitBreaker, NewMockProvider(), Provider, defaultMockProviders(), newCircuitBreaker(), NewDefaultMockRoundRobinSender(), newMemoryProviderState(), NewRoundRobinSender() (+18 more)

### Community 3 - "Community 3"
Cohesion: 0.1
Nodes (20): main(), run(), Config, getEnv(), Load(), DatabaseConfig, ServerConfig, NewDispatcher() (+12 more)

### Community 4 - "Community 4"
Cohesion: 0.09
Nodes (13): BalanceCache, toWalletModel(), toWalletTransactionModel(), BaseModel, ChannelModel, PostgresWalletRepository, SMSCostModel, SMSDeliveryModel (+5 more)

### Community 5 - "Community 5"
Cohesion: 0.12
Nodes (17): ChannelResponse, CreateChannelRequest, CreateWalletRequest, ToChannelResponse(), ToSMSDeliveryResponse(), ToTransactionResponse(), ToWalletResponse(), SMSDeliveryResponse (+9 more)

### Community 6 - "Community 6"
Cohesion: 0.14
Nodes (11): Dispatcher, Declare(), NewWorker(), NewWorkerWithReservation(), NewWorkerWithReservationAndTimeout(), selectFairRows(), Sender, Worker (+3 more)

### Community 7 - "Community 7"
Cohesion: 0.1
Nodes (14): toSMSCostModel(), NewSMSDeliveryRepository(), toDeliveryReport(), PostgresSMSCostRepository, NewPostgresSMSCostRepository(), SMSDeliveryRepository, DeliveryReport, Destination (+6 more)

### Community 8 - "Community 8"
Cohesion: 0.15
Nodes (12): ChannelFinder, DeliveryEvent, ReservationStore, Sender, SendSMSCommand, SendSMSResult, Service, generateMessageID() (+4 more)

### Community 9 - "Community 9"
Cohesion: 0.24
Nodes (5): BalanceLoader, balanceKey(), NewStore(), reservationKey(), Store

### Community 10 - "Community 10"
Cohesion: 0.17
Nodes (11): API documentation, code:sh (docker compose -f deployments/docker/compose.yaml up --build), code:sh (ADMIN_API_KEY=change-me POSTGRES_PASSWORD=change-me APP_PORT), code:sh (EXPRESS_WORKER_REPLICAS=6 NORMAL_WORKER_REPLICAS=2 \), code:sh (go test -race ./...), Configuration, Delivery guarantees, Docker Compose (+3 more)

### Community 11 - "Community 11"
Cohesion: 0.24
Nodes (3): circuitState, memoryProviderState, RoundRobinSender

### Community 12 - "Community 12"
Cohesion: 0.38
Nodes (4): newRedisProviderState(), redisCircuitState(), redisInt64(), redisProviderState

### Community 13 - "Community 13"
Cohesion: 0.31
Nodes (6): requireAPIKey(), Setup(), TestAdminRoutesRequireAPIKey(), TestAdminRoutesRequireAPIKey(), TestSMSReportRoutesRequireAPIKey(), TestSwaggerUIIsPublic()

### Community 14 - "Community 14"
Cohesion: 0.38
Nodes (3): toChannelModel(), NewPostgresChannelRepository(), PostgresChannelRepository

### Community 15 - "Community 15"
Cohesion: 0.29
Nodes (6): Boundaries, Channel isolation and SLA, code:text (HTTP router/handler -> application services -> domain rules), Diagrams, Failure and retry behavior, SMS Gateway Architecture

### Community 16 - "Community 16"
Cohesion: 0.4
Nodes (4): SwaggerDeliveryReport, SwaggerErrorResponse, SwaggerSMSRequest, SwaggerSMSResponse

## Knowledge Gaps
- **60 isolated node(s):** `Config`, `ServerConfig`, `DeliveryReport`, `SMSCost`, `SMSCostRepository` (+55 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **13 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `run()` connect `Community 3` to `Community 0`, `Community 1`, `Community 4`, `Community 5`, `Community 6`, `Community 7`, `Community 9`, `Community 13`, `Community 14`?**
  _High betweenness centrality (0.234) - this node is a cross-community bridge._
- **Why does `MustMoney()` connect `Community 0` to `Community 1`, `Community 3`, `Community 4`, `Community 5`?**
  _High betweenness centrality (0.220) - this node is a cross-community bridge._
- **Why does `main()` connect `Community 6` to `Community 9`, `Community 2`, `Community 3`?**
  _High betweenness centrality (0.145) - this node is a cross-community bridge._
- **Are the 15 inferred relationships involving `run()` (e.g. with `Load()` and `Connect()`) actually correct?**
  _`run()` has 15 INFERRED edges - model-reasoned connections that need verification._
- **Are the 12 inferred relationships involving `MustMoney()` (e.g. with `toWalletModel()` and `.DeductAndEnqueue()`) actually correct?**
  _`MustMoney()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `Load()` (e.g. with `main()` and `main()`) actually correct?**
  _`Load()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **Are the 6 inferred relationships involving `NewRoundRobinSender()` (e.g. with `NewRedisRoundRobinSender()` and `TestRoundRobinSenderDistributesRequestsInOrder()`) actually correct?**
  _`NewRoundRobinSender()` has 6 INFERRED edges - model-reasoned connections that need verification._