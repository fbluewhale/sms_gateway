# Deployments

`compose.yaml` runs PostgreSQL, RabbitMQ, the API, one outbox dispatcher, and
independently scalable Express and Normal worker pools. The Docker build uses
the host proxy default `http://127.0.0.1:10808`.

Scale lines independently with `EXPRESS_WORKER_REPLICAS` and
`NORMAL_WORKER_REPLICAS`. Set production passwords, API keys, broker
credentials, and SLA/capacity limits through environment variables.

The Draw.io files in this directory document the component architecture and
SMS delivery sequence.
