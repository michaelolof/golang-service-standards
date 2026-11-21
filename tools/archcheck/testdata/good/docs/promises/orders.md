# Orders: what callers are promised

**Callers:** the tenant's app, with a key that has `orders:write`; and an operator.
**Sources:** the API description, and `docs/features/orders.md`.
**Operations:** `POST /v1/orders/{id}/cancel`

| ID | Promise | Varies by | Edge cases a caller can meet | Proved by |
|---|---|---|---|---|
| ORD-1 | An operator sees an order the tenant's app placed. | — | — | `TestPlaceOrder_AnOperatorSeesTheOrder` |

## Not reachable from outside

- **A read-only key cancelling an order.** No key a caller can be issued today may read without writing, so no caller can reach this refusal. Its handler test covers it: `TestCancelOrderHandler_RefusesAKeyThatMayOnlyRead`.

## Not yet proved as the caller

- **ORD-1.** `TestPlaceOrder_AnOperatorSeesTheOrder` seeds its orders straight into the table, where the tenant's app places them through the API. It's rewritten when TEST-14 is lifted.
