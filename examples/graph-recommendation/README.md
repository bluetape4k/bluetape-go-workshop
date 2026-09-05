# Graph Recommendation

[English](README.md) | [한국어](README.ko.md)

This `v0.10.0` workshop example implements issue #51 with the caller-owned
[`graph`](https://github.com/bluetape4k/bluetape-go/tree/develop/graph) model.
It keeps traversal, scoring, validation, and output projection in the
application package; it does not add a backend, query DSL, or recommender
framework.

## Lesson boundary

The example answers one narrow question: how can an application turn a small
validated graph into explainable product and follow candidates with stable
ordering?

- `User` and `Product` vertices carry `fixture_id` plus validated display
  properties.
- `PURCHASED` edges point from a user to a product and carry an integer
  `rating` from 1 to 5.
- `FOLLOWS` edges point from one user to another user.
- `graph.Path` stores each evidence walk. The application owns endpoint
  semantics because the library model validates values and step shape, not a
  backend traversal query.

## Traversal and scoring

### Product recommendations

For seed `alice`, the application performs this bounded traversal:

```text
seed user -> PURCHASED products -> incoming PURCHASED co-buyers
          -> co-buyers' PURCHASED products
```

The score is the number of distinct co-buyers that unlock a candidate. Products
already purchased by the seed are excluded. Results are ordered by `score`
descending, then `product_id` ascending. One deterministic shared product is
kept for each co-buyer/candidate pair so the JSON path remains reproducible.
The incoming co-buyer purchase is shown in the evidence sequence even though
the stored `PURCHASED` edge direction remains `User -> Product`.

### Follow recommendations

The FOAF traversal is two outgoing `FOLLOWS` hops:

```text
seed user -> direct follows -> second-hop candidates
```

The score is the number of distinct direct follows that reach a candidate. The
seed and already-followed users are excluded. Results use the same score
descending, identifier ascending ordering.

## Deterministic fixture

The `graph-recommendation-v1` fixture contains six users, six products, 13
`PURCHASED` edges, and 12 `FOLLOWS` edges. Running the CLI for `alice` produces three product candidates
and two follow candidates:

```json
{
  "seed_user": "alice",
  "product_recommendations": [
    {
      "product_id": "headphones",
      "score": 3,
      "evidence": [
        {
          "co_buyer_id": "bob",
          "shared_product_id": "laptop",
          "path": [
            "alice",
            "laptop",
            "bob",
            "headphones"
          ]
        },
        {
          "co_buyer_id": "carol",
          "shared_product_id": "phone",
          "path": [
            "alice",
            "phone",
            "carol",
            "headphones"
          ]
        },
        {
          "co_buyer_id": "dave",
          "shared_product_id": "tablet",
          "path": [
            "alice",
            "tablet",
            "dave",
            "headphones"
          ]
        }
      ]
    },
    {
      "product_id": "keyboard",
      "score": 1,
      "evidence": [
        {
          "co_buyer_id": "eve",
          "shared_product_id": "laptop",
          "path": [
            "alice",
            "laptop",
            "eve",
            "keyboard"
          ]
        }
      ]
    },
    {
      "product_id": "mouse",
      "score": 1,
      "evidence": [
        {
          "co_buyer_id": "frank",
          "shared_product_id": "phone",
          "path": [
            "alice",
            "phone",
            "frank",
            "mouse"
          ]
        }
      ]
    }
  ],
  "follow_recommendations": [
    {
      "user_id": "dave",
      "score": 1,
      "evidence": [
        {
          "via_user_id": "bob",
          "path": [
            "alice",
            "bob",
            "dave"
          ]
        }
      ]
    },
    {
      "user_id": "eve",
      "score": 1,
      "evidence": [
        {
          "via_user_id": "carol",
          "path": [
            "alice",
            "carol",
            "eve"
          ]
        }
      ]
    }
  ]
}
```

## Run

Print the local report:

```bash
go run ./examples/graph-recommendation
```

Run focused tests, including validation, empty candidates, tie-breaking,
record-order independence, path evidence, and cancellation:

```bash
go test -count=1 ./examples/graph-recommendation/...
go test -race -count=1 ./examples/graph-recommendation/...
```

## Scope and production boundary

This is a deterministic teaching fixture, not a production recommender. It
does not claim relevance, personalization quality, privacy policy compliance,
or online serving performance. A real service must own identity, consent,
freshness, ranking policy, pagination, persistence, and backend-specific query
planning outside this example.
