# Orders: what callers are promised

**Callers:** the tenant's app, with a key that has `orders:write` and `orders:read`; an operator; and the payment provider's webhooks.
**Sources:** the API description, and `docs/features/orders.md`.
**Operations:** `POST /api/v1/orders` · `GET /api/v1/orders/{order_id}` · `POST /api/v1/orders/{order_id}/cancel` · `GET /admin/orders/{order_id}`

| ID | Promise | Varies by | Edge cases a caller can meet | Proved by |
|---|---|---|---|---|
| ORD-1 | Each kind of customer can place an order and pay it. It's `PLACED`, then `PAID` once the provider settles it, and the app is told `ORDER_PLACED`, then `ORDER_PAID`. | person, business · each country · each currency | the provider times out: 503, and the order stays `PLACED` | `TestOrder_EachKindOfCustomerPlacesAnOrderAndPaysIt` |

## Not reachable from outside

- **`FAILED`.** The provider never refuses a payment for good once it has accepted it, so no caller can reach this status. Its rule is a core test: `TestChargeOrder_MarksTheOrderFailedWhenTheProviderRefusesForGood`.

## Not yet proved as the caller

Only while TEST-14's pause holds. Delete this section otherwise. One entry per existing flow that doesn't act as a caller yet: the promise, the flow, and what it does that a caller can't. The lift empties it. For example, if ORD-1's flow still seeded its customer:

- **ORD-1.** `TestOrder_EachKindOfCustomerPlacesAnOrderAndPaysIt` seeds its customer straight into the table, always as a person, where the tenant's app verifies one through the API.
