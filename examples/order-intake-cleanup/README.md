# order-intake-cleanup

Partner order feed cleanup example for `core` and `collections`.

The example validates required fields, defaults optional notes/coupons, filters
bad line items, deduplicates tags, and groups accepted orders by sales channel.
It intentionally keeps ordinary `for` loops where they are clearer than helper
calls.
