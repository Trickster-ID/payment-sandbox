# API Contract v1 (Backend)

Base path: `/api/v1`  
Response envelope:

```json
{
  "data": {},
  "meta": {},
  "error": {
    "code": "string",
    "message": "string",
    "details": {}
  }
}
```

Notes:
- Success responses use `data` (optional `meta`).
- Error responses use `error` with stable `code`.
- `X-Request-ID` is propagated by middleware.
- All datetime fields in request/response payloads use RFC3339 (ISO 8601 profile) in UTC where applicable.
- Monetary values in v1 are interpreted as `IDR` (ISO 4217 single-currency policy).

## Public Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/ping` | No | Health check |
| POST | `/users/register` | No | Register merchant |
| GET | `/pay/:token` | No | Public invoice detail |
| POST | `/pay/:token/intents` | No | Create payment intent |

## Merchant Endpoints (`MERCHANT`)

| Method | Path | Description |
|---|---|---|
| GET | `/merchant/wallet` | Get wallet balance |
| POST | `/merchant/topups` | Create top-up request |
| POST | `/merchant/invoices` | Create invoice |
| GET | `/merchant/invoices` | List invoices (supports `status`, `page`, `limit`) |
| GET | `/merchant/invoices/:id` | Get invoice detail |
| POST | `/merchant/refunds` | Request refund |

## Admin Endpoints (`ADMIN`)

| Method | Path | Description |
|---|---|---|
| GET | `/admin/topups` | List top-ups |
| PATCH | `/admin/topups/:id/status` | Update top-up status |
| GET | `/admin/payment-intents` | List payment intents |
| PATCH | `/admin/payment-intents/:id/status` | Update payment intent status |
| GET | `/admin/refunds` | List refunds |
| PATCH | `/admin/refunds/:id/review` | Review refund (approve/reject) |
| PATCH | `/admin/refunds/:id/process` | Process refund status |
| GET | `/admin/stats` | Dashboard stats |

## Common Error Codes

| Code | HTTP | Typical Context |
|---|---:|---|
| `validation_error` | 400 | Request payload validation failed |
| `auth_missing_bearer_token` | 401 | Missing bearer token |
| `auth_invalid_token` | 401 | Invalid JWT |
| `auth_unauthorized` | 401 | Missing user context |
| `auth_forbidden` | 403 | Role not allowed |
| `invoice_not_found` | 404 | Invoice not found |
| `wallet_not_found` | 404 | Wallet not found |
| `*_failed` | 400 | Domain-specific command failure |
