# S3 Floci Storage Example Lesson

## Context

Issue #59 adds the first S3-shaped storage example for the v0.7.0 workshop
track. The goal is not to wrap the whole AWS SDK. The useful lesson is the
small application boundary around receipt objects: safe key construction,
metadata ownership, body close responsibility, typed missing-object mapping,
tenant prefix listing, and caller-owned presigned GET URLs.

## Decision

The example keeps the storage API narrow and passes caller-owned AWS SDK v2
clients into each operation. Local smoke coverage uses `testcontainers/floci`
with path-style addressing and endpoint override, while the README explains
that real AWS uses the same request shapes with normal credentials, IAM,
networking, encryption, and observability controls supplied outside the example.

## Rejected

- A broad repository wrapper over every S3 feature. It would hide the AWS SDK
  contract the reader needs to understand.
- Real AWS integration tests in the workshop gate. They would require cloud
  credentials and produce non-deterministic cost and account-state concerns.
- Mixing S3 storage with SQS or DynamoDB in the same example. S3 object storage
  is the prerequisite lesson and should stay readable before event or index
  projections are introduced.

## Verification Shape

- Fake-client tests prove request shape, metadata, safe-key validation,
  `NoSuchKey` mapping, nil body defense, body closing, delete, list, and
  presign TTL behavior without Docker.
- The opt-in Floci smoke test proves AWS SDK v2 path-style S3 calls against a
  local emulator when Docker is available.
- README diagrams split static ownership boundaries from the operation
  sequence so readers can understand both local Floci and real AWS deployment
  differences.
