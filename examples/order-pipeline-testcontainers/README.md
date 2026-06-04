# order-pipeline-testcontainers

Order pipeline integration example for `testcontainers`.

The test starts PostgreSQL, Redis, and NATS through `bluetape-go` fixtures,
uses returned connection strings instead of hard-coded ports, writes a small
order projection, stores an idempotency marker, and publishes a lightweight
event. Docker is required for this example.
