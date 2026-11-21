# Orders: what callers are promised

**Callers:** the tenant's app, with a key that has `orders:write` and `orders:read`; an operator; and the payment provider's webhooks.
**Sources:** the API description, and `docs/features/orders.md`.
**Operations:** `POST /api/v1/orders` · `GET /api/v1/orders/{order_id}` · `POST /api/v1/orders/{order_id}/cancel` · `GET /admin/orders/{order_id}`

| ID | Promise | Varies by | Edge cases a caller can meet | Proved by |
|---|---|---|---|---|
| ORD-1 | Each kind of customer can place an order and pay it. It's `PLACED`, then `PAID` once the provider settles it, and the app is told `ORDER_PLACED`, then `ORDER_PAID`. | person, business · each country · each currency | the provider times out: 503, and the order stays `PLACED` | `TestOrder_EachKindOfCustomerPlacesAnOrderAndPaysIt` |

## Not reachable from outside

- **`FAILED`.** The provider never refuses a payment for good once it has accepted it, so no caller can reach this status. Its rule is a core test: `TestChargeOrder_MarksTheOrderFailedWhenTheProviderRefusesForGood`.
