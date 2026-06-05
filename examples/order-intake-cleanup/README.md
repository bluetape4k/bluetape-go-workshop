# order-intake-cleanup

[English](README.md) | [한국어](README.ko.md)

Partner order feed cleanup example for `core` and `collections`.

The example validates required fields, defaults optional notes/coupons, filters
bad line items, deduplicates tags, and groups accepted orders by sales channel.
It intentionally keeps ordinary `for` loops where they are clearer than helper
calls.

## Scenario

![Data codec and cleanup flow](../../docs/images/readme-diagrams/data-codec-cleanup-flow.png)

Use this example when a partner feed needs deterministic cleanup before it can
enter a domain pipeline. The code keeps validation, defaults, filtering,
deduplication, and grouping as visible steps instead of hiding the whole feed in
one helper call.

## What It Demonstrates

- Required-field validation with `core` helpers.
- Defaulting optional notes and coupons.
- Filtering unusable line items before grouping.
- Deduplicating tags while preserving stable output.
- Grouping accepted orders by sales channel.

## Run

```bash
go test -count=1 ./examples/order-intake-cleanup/...
```
