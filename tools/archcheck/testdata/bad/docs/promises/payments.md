# Payments: what callers are promised

**Callers:** the tenant's app; and the provider's webhooks.
**Sources:** the API description.
**Operations:** `GET /v1/transfers/{id}`

| ID | Promise | Varies by | Edge cases a caller can meet | Proved by |
|---|---|---|---|---|
| PAY-1 | A settled payment shows as settled. | — | — | `TestPayment_ASettledPaymentShowsAsSettled` |
| PAY-2 | A refused payment says why. | — | — | `TestPayment_ARefusedPaymentSaysWhy` |
| PAY-3 | A payment is never taken twice. | — | — | `TestRepo_StoresATransfer` |
| PAY-4 | A payment is refused without an account. | — | — | — |
| PAY-1 | A payment can be read by its ID. | — | — | `TestPayment_ASettledPaymentShowsAsSettled` |

## Not reachable from outside

- **`FAILED`.** `TestChargeOrder_MarksTheOrderFailed`
- **`REVERSED`.** The provider never reverses a settled payment, so no caller can reach this status.

## Not yet proved as the caller

- **PAY-1.** `TestPayment_SettlesThroughTheFake` stands a fake in for the provider.
