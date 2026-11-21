# Transfers: what callers are promised

**Callers:** the tenant's app, with a key that has `transfers:read`.
**Sources:** the API description.
**Operations:** `GET /v1/transfers/{id}`

ID | Promise | Varies by | Edge cases a caller can meet | Proved by
---|---|---|---|---
TRF-1 | The tenant's app reads a transfer by its ID, and its details are kept nowhere else. | — | another tenant's transfer: 404 \| a deleted one: 404 | `TestTransfer_TheAppReadsATransferByItsID`
