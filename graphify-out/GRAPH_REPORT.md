# Graph Report - .  (2026-07-12)

## Corpus Check
- Corpus is ~4,101 words - fits in a single context window. You may not need a graph.

## Summary
- 162 nodes · 202 edges · 18 communities (13 shown, 5 thin omitted)
- Extraction: 78% EXTRACTED · 22% INFERRED · 0% AMBIGUOUS · INFERRED: 45 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

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
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]

## God Nodes (most connected - your core abstractions)
1. `run()` - 13 edges
2. `Money` - 9 edges
3. `SMSHandler` - 8 edges
4. `writeServiceError()` - 8 edges
5. `NewMoney()` - 7 edges
6. `PostgresWalletRepository` - 7 edges
7. `AdminService` - 7 edges
8. `MustMoney()` - 6 edges
9. `LineType` - 5 edges
10. `NewService()` - 5 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `NewPostgresChannelRepository()`  [INFERRED]
  cmd/api-gateway/main.go → internal/infrastructure/persistence/channel_repository.go
- `run()` --calls--> `NewPostgresWalletRepository()`  [INFERRED]
  cmd/api-gateway/main.go → internal/infrastructure/persistence/wallet_repository.go
- `run()` --calls--> `NewPostgresSMSCostRepository()`  [INFERRED]
  cmd/api-gateway/main.go → internal/infrastructure/persistence/sms_cost_repository.go
- `run()` --calls--> `Load()`  [INFERRED]
  cmd/api-gateway/main.go → internal/config/config.go
- `run()` --calls--> `Connect()`  [INFERRED]
  cmd/api-gateway/main.go → internal/infrastructure/persistence/database.go

## Communities (18 total, 5 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.15
Nodes (12): ChannelResponse, CreateChannelRequest, CreateWalletRequest, ToChannelResponse(), ToTransactionResponse(), ToWalletResponse(), TopUpWalletRequest, TransactionResponse (+4 more)

### Community 1 - "Community 1"
Cohesion: 0.13
Nodes (11): NewAdminService(), main(), run(), NewSMSHandler(), Connect(), Migrate(), requireAPIKey(), Setup() (+3 more)

### Community 2 - "Community 2"
Cohesion: 0.13
Nodes (9): toSMSCostModel(), PostgresSMSCostRepository, NewPostgresSMSCostRepository(), Destination, ToCommand(), ToResponse(), LineType, SendSMSRequest (+1 more)

### Community 3 - "Community 3"
Cohesion: 0.17
Nodes (3): Money, Wallet, WalletTransaction

### Community 4 - "Community 4"
Cohesion: 0.18
Nodes (9): channelStub, costStub, senderStub, NewService(), TestExecuteChargesAndSends(), TestExecuteRejectsInvalidCommandBeforeDependencies(), TestExecuteReturnsSenderFailure(), walletStub (+1 more)

### Community 5 - "Community 5"
Cohesion: 0.15
Nodes (7): toWalletTransactionModel(), BaseModel, ChannelModel, SMSCostModel, WalletModel, WalletTransactionModel, TransactionType

### Community 6 - "Community 6"
Cohesion: 0.2
Nodes (4): AdminService, NewMoney(), TestMoneySubtract(), TestNewMoney()

### Community 7 - "Community 7"
Cohesion: 0.33
Nodes (3): toWalletModel(), PostgresWalletRepository, NewPostgresWalletRepository()

### Community 8 - "Community 8"
Cohesion: 0.25
Nodes (7): ChannelFinder, Sender, SendSMSCommand, SendSMSResult, Service, generateMessageID(), WalletPayable

### Community 9 - "Community 9"
Cohesion: 0.33
Nodes (5): Config, getEnv(), Load(), DatabaseConfig, ServerConfig

### Community 10 - "Community 10"
Cohesion: 0.38
Nodes (3): toChannelModel(), NewPostgresChannelRepository(), PostgresChannelRepository

## Knowledge Gaps
- **23 isolated node(s):** `Config`, `ServerConfig`, `SMSCost`, `SMSCostRepository`, `Channel` (+18 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **5 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `run()` connect `Community 1` to `Community 2`, `Community 4`, `Community 7`, `Community 9`, `Community 10`?**
  _High betweenness centrality (0.001) - this node is a cross-community bridge._
- **Why does `MustMoney()` connect `Community 4` to `Community 6`?**
  _High betweenness centrality (0.001) - this node is a cross-community bridge._
- **Are the 11 inferred relationships involving `run()` (e.g. with `Load()` and `Connect()`) actually correct?**
  _`run()` has 11 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Config`, `ServerConfig`, `SMSCost` to the rest of the system?**
  _23 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.13 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.13 - nodes in this community are weakly interconnected._