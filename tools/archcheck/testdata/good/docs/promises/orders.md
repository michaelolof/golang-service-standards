# Orders: what callers are promised

**Callers:** the tenant's app, with a key that has `orders:write`; and an operator.
**Sources:** the API description, and `docs/features/orders.md`.
**Operations:** `POST /v1/orders/{id}/cancel`

| ID | Promise | Varies by | Edge cases a caller can meet | Proved by |
|---|---|---|---|---|
| ORD-1 | An operator sees an order the tenant's app placed. | — | — | `TestPlaceOrder_AnOperatorSeesTheOrder` |

## Not reachable from outside ##

1. **A read-only key cancelling an order.** No key a caller can be issued today may read without writing, so no caller can reach this refusal. Its handler test covers it: `TestCancelOrderHandler_RefusesAKeyThatMayOnlyRead`.
   - A nested item belongs to the entry above it.
