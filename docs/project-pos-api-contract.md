# Intania Shop Project/POS API Contract

Status: Draft for product, frontend, and backend review  
Last updated: 26 August 2026  
Canonical machine-readable contract: [`openapi.yaml`](./openapi.yaml)

This document is a human-readable review companion for the Project/POS additions in the OpenAPI contract. If this document and OpenAPI disagree, `openapi.yaml` is the source of truth.

## 1. Product flow

The API supports the following event-sale workflow:

```text
Create project
  -> select products and variants
  -> define project-specific prices
  -> create bundle promotions
  -> open POS catalogue
  -> quote cart on the server
  -> accept QR or cash payment
  -> atomically create order and reduce stock
  -> review and export project orders
```

The backend has been reduced to admin-only authentication, catalogue, inventory, and uploads. Customer carts and orders, image promos, banners, bulk-sale inventory, and product videos were removed before Project/POS implementation. Project/POS will reuse the core products, variants, users, stock, and inventory transaction ledger.

## 2. Shared conventions

### Access control

All endpoints in this document require an `ADMIN` JWT:

```http
Authorization: Bearer <admin-token>
```

No new staff role is introduced for the MVP. Staff identity is taken from JWT claims and never accepted from checkout request bodies.

### Response envelopes

Successful JSON response:

```json
{
  "success": true,
  "data": {}
}
```

Failed Project/POS response:

```json
{
  "success": false,
  "error": "Requested quantity exceeds available stock",
  "code": "INSUFFICIENT_STOCK",
  "details": {
    "items": [
      {
        "product_id": 10,
        "variant_id": 22,
        "requested_quantity": 4,
        "available_quantity": 2
      }
    ]
  },
  "request_id": "req_01J6B8K8QF4C"
}
```

### IDs, money, dates, and pagination

- Resource IDs are positive `int64` values.
- THB money is a JSON string with exactly two decimals, such as `"299.00"`.
- JSON numeric money values are invalid.
- Project dates use `YYYY-MM-DD`.
- Timestamps use RFC 3339.
- Project status uses the current date in `Asia/Bangkok` and includes both boundary dates.
- Pagination uses `page` and `page_size`, defaulting to `1` and `10`.
- Maximum `page_size` is `100`.

## 3. Project lifecycle

Project status is derived and is never accepted in create or update requests:

| Status        | Meaning                                                        |
| ------------- | -------------------------------------------------------------- |
| `NOT_STARTED` | Bangkok date is before `start_date`                            |
| `ACTIVE`      | Bangkok date is between `start_date` and `end_date`, inclusive |
| `COMPLETED`   | Bangkok date is after `end_date`                               |

An omitted or null `end_date` creates a single-day project and is normalized to `start_date` in responses.

Lifecycle rules:

- Name and description remain correctable after sales exist.
- Start and end dates become immutable after the first project order.
- Products, prices, promotions, and category order may change while `NOT_STARTED` or `ACTIVE`.
- Sales configuration is locked when `COMPLETED`.
- A project can be deleted only while it has no orders.
- Deleting an eligible project also deletes its project-specific configuration, never the master products.
- Confirmed POS orders cannot be edited, cancelled, refunded, or deleted in the MVP.

## 4. Endpoint summary

### Projects

| Method   | Endpoint                 | Purpose                                     |
| -------- | ------------------------ | ------------------------------------------- |
| `GET`    | `/projects`              | List, filter, and paginate projects         |
| `POST`   | `/projects`              | Create a project                            |
| `GET`    | `/projects/{project_id}` | Get project details                         |
| `PUT`    | `/projects/{project_id}` | Update project metadata and permitted dates |
| `DELETE` | `/projects/{project_id}` | Delete a project that has no orders         |

`GET /projects` accepts:

- `name`: case-insensitive partial match
- `status`: `NOT_STARTED`, `ACTIVE`, or `COMPLETED`
- `page`
- `page_size`

Filters are applied before pagination. Results are stably ordered by newest `created_at`, then highest project ID.

Create/update example:

```json
{
  "name": "Engineering Fair 2026",
  "description": "Main merchandise booth",
  "start_date": "2026-09-05",
  "end_date": "2026-09-07"
}
```

Project response:

```json
{
  "project_id": 7,
  "name": "Engineering Fair 2026",
  "description": "Main merchandise booth",
  "start_date": "2026-09-05",
  "end_date": "2026-09-07",
  "status": "NOT_STARTED",
  "order_count": 0,
  "created_at": "2026-08-26T10:00:00+07:00",
  "updated_at": "2026-08-26T10:00:00+07:00"
}
```

### Project products and categories

| Method | Endpoint                                           | Purpose                                      |
| ------ | -------------------------------------------------- | -------------------------------------------- |
| `GET`  | `/projects/{project_id}/product-candidates`        | Paginated product picker data                |
| `POST` | `/projects/{project_id}/product-selection/preview` | Resolve all products matching picker filters |
| `GET`  | `/projects/{project_id}/products`                  | List assigned products and variants          |
| `PUT`  | `/projects/{project_id}/products`                  | Atomically replace all assignments           |
| `GET`  | `/projects/{project_id}/categories`                | List categories and selected-product counts  |

Product candidates accept `name`, `category`, `page`, and `page_size`. Each product contains one or more sellable items with stock, default price, selected state, and current project price.

The preview endpoint accepts the same name/category filter but does not mutate data. It returns all matching sellable item references so the frontend can implement Select All across pagination.

`PUT /projects/{project_id}/products` is a complete replacement, not a patch:

```json
{
  "items": [
    {
      "product_id": 10,
      "variant_id": 22,
      "project_price": "299.00"
    },
    {
      "product_id": 11,
      "variant_id": null,
      "project_price": "59.00"
    }
  ]
}
```

Rules:

- A product with variants requires a valid selected `variant_id`.
- A genuinely variantless product uses `variant_id: null`.
- `project_price` must be non-negative.
- Duplicate sellable item references are invalid.
- The entire replacement succeeds or fails in one transaction.
- Removing an assignment never deletes the master product or variant.

### Project pricing promotions

| Method   | Endpoint                                           | Purpose                    |
| -------- | -------------------------------------------------- | -------------------------- |
| `GET`    | `/projects/{project_id}/promotions`                | List pricing promotions    |
| `POST`   | `/projects/{project_id}/promotions`                | Create a pricing promotion |
| `GET`    | `/projects/{project_id}/promotions/{promotion_id}` | Get a pricing promotion    |
| `PUT`    | `/projects/{project_id}/promotions/{promotion_id}` | Update a pricing promotion |
| `DELETE` | `/projects/{project_id}/promotions/{promotion_id}` | Delete a pricing promotion |

These are the only promotion concept in the planned backend. The former image-based `/promos` API has been removed.

Mutation example:

```json
{
  "name": "Shirt and pin set",
  "promotion_price": "329.00",
  "items": [
    {
      "product_id": 10,
      "variant_id": 22,
      "quantity": 1
    },
    {
      "product_id": 11,
      "variant_id": null,
      "quantity": 1
    }
  ]
}
```

Rules:

- Every item must currently be assigned to the same project.
- Quantities are positive integers.
- A sellable item may appear only once in a promotion.
- Promotion price must be between zero and the current original bundle price.
- Original bundle price and discount are server-derived from current project prices.
- At most one promotion is applied to a cart, and it is applied at most once.
- The eligible promotion with the largest discount wins.
- Equal discounts choose the oldest `created_at`, then the lowest promotion ID.
- Order snapshots preserve the applied promotion even if configuration later changes.

### POS catalogue and category order

| Method | Endpoint                                    | Purpose                                                              |
| ------ | ------------------------------------------- | -------------------------------------------------------------------- |
| `GET`  | `/projects/{project_id}/pos`                | Get project summary, ordered categories, products, prices, and stock |
| `PUT`  | `/projects/{project_id}/pos/category-order` | Replace the complete category order                                  |

The POS catalogue is available in every project status for preview. Its `can_checkout` field is true only while `ACTIVE`.

Category order request:

```json
{
  "categories": ["Apparel", "Accessories"]
}
```

The request must contain every current project category exactly once. Unknown, duplicated, or omitted categories make the full update fail.

### Quotation

| Method | Endpoint                                | Purpose                                           |
| ------ | --------------------------------------- | ------------------------------------------------- |
| `POST` | `/projects/{project_id}/checkout/quote` | Validate stock and calculate authoritative totals |

Quotation is available only while the project is `ACTIVE`.

```json
{
  "items": [
    {
      "product_id": 10,
      "variant_id": 22,
      "quantity": 2
    },
    {
      "product_id": 11,
      "variant_id": null,
      "quantity": 1
    }
  ]
}
```

The response resolves product snapshots and returns:

- line-item unit price and total
- available quantity
- subtotal
- nullable applied promotion
- discount
- net total
- quotation timestamp

The API never accepts price, promotion, discount, subtotal, or net total from the client.

Draft POS carts are frontend-owned. The frontend keeps one cart per project in `sessionStorage`; no new server cart endpoints are included.

### Payment slip upload

| Method | Endpoint                | Purpose                             |
| ------ | ----------------------- | ----------------------------------- |
| `POST` | `/upload/payment-slips` | Upload one trusted QR payment image |

The multipart field name is `file`. Supported types are JPEG, PNG, and WebP, with a maximum size of 10 MiB.

```json
{
  "success": true,
  "data": {
    "object_key": "payment-slips/2026/08/example.png",
    "url": "https://storage.googleapis.com/intania-shop/payment-slips/2026/08/example.png",
    "content_type": "image/png",
    "size": 482193
  }
}
```

QR checkout sends the trusted `object_key`; it does not accept an arbitrary slip URL.

### Atomic POS checkout

| Method | Endpoint                        | Purpose                                         |
| ------ | ------------------------------- | ----------------------------------------------- |
| `POST` | `/projects/{project_id}/orders` | Create a paid order and reduce stock atomically |

Checkout requires a UUID header generated once per user confirmation attempt:

```http
Idempotency-Key: 82e88926-0450-42d4-b3f6-502c3d9d0a4b
```

Buyer rules:

- `gender` is required: `MALE`, `FEMALE`, or `PREFER_NOT_TO_SAY`.
- `age` is optional; when present it is a non-negative integer.
- `student_alumni_year` is optional trimmed cohort text up to 50 characters.

QR checkout:

```json
{
  "buyer": {
    "gender": "FEMALE",
    "age": 21,
    "student_alumni_year": "Intania 105"
  },
  "items": [
    {
      "product_id": 10,
      "variant_id": 22,
      "quantity": 1
    }
  ],
  "payment": {
    "method": "QR_CODE",
    "slip_object_key": "payment-slips/2026/08/example.png",
    "note": "Paid at booth 1"
  }
}
```

Cash checkout:

```json
{
  "buyer": {
    "gender": "PREFER_NOT_TO_SAY",
    "age": 0,
    "student_alumni_year": null
  },
  "items": [
    {
      "product_id": 11,
      "variant_id": null,
      "quantity": 2
    }
  ],
  "payment": {
    "method": "REAL_MONEY",
    "received_amount": "500.00",
    "no_change": true,
    "note": null
  }
}
```

Cash rules:

- `received_amount` must be at least the recalculated net total.
- Normal payment returns `received_amount - net_total` as change.
- With `no_change: true`, `change_amount` is zero and the excess is recorded as `retained_amount`.

Checkout behavior:

- Project must be `ACTIVE`.
- Prices and promotions are recalculated using the same pricing logic as quotation.
- Stock rows are locked and cannot become negative under concurrent checkout.
- Order, payment, item snapshots, stock reductions, and inventory history commit or roll back together.
- Successful booth orders have status `COMPLETED`.
- An identical retry with the same idempotency key returns the original order with HTTP `200` and `Idempotency-Replayed: true`.
- The initial creation returns HTTP `201`.
- Reusing a key for different content returns `409 IDEMPOTENCY_KEY_REUSED`.

### Order history, detail, and export

| Method | Endpoint                                   | Purpose                               |
| ------ | ------------------------------------------ | ------------------------------------- |
| `GET`  | `/projects/{project_id}/orders`            | Filter, sort, and paginate POS orders |
| `GET`  | `/projects/{project_id}/orders/{order_id}` | Get immutable order details           |
| `GET`  | `/projects/{project_id}/orders/export`     | Export the filtered result as `.xlsx` |

History and export filters:

- `order_number`
- `payment_method`: `QR_CODE` or `REAL_MONEY`
- `staff_id`
- `created_from`: RFC 3339
- `created_to`: RFC 3339
- `sort_by`: `order_number` or `net_total`
- `sort_order`: `asc` or `desc`

History additionally accepts `page` and `page_size`. The default date range starts at project start `00:00:00 Asia/Bangkok` and ends at the current time.

Order detail contains immutable staff, buyer, item, price, promotion, totals, payment, QR slip, and inventory transaction snapshots. `order_number` is opaque and must not be parsed by clients.

Export is synchronous and uses the same filters and sorting without pagination. The workbook contains one row per order item and repeats order-level fields so multi-item orders lose no information.

## 5. Stable error codes

| Code                      | Typical status | Meaning                                                             |
| ------------------------- | -------------: | ------------------------------------------------------------------- |
| `VALIDATION_ERROR`        |            400 | Request fields, filters, or pagination are invalid                  |
| `AUTHENTICATION_REQUIRED` |            401 | JWT is missing or invalid                                           |
| `FORBIDDEN`               |            403 | Caller is authenticated but is not an admin                         |
| `PROJECT_NOT_FOUND`       |            404 | Project does not exist                                              |
| `PROMOTION_NOT_FOUND`     |            404 | Project promotion does not exist                                    |
| `PROJECT_NOT_ACTIVE`      |            409 | Quotation or checkout attempted outside the sale window             |
| `PROJECT_COMPLETED`       |            409 | Completed project configuration cannot be changed                   |
| `PROJECT_HAS_ORDERS`      |            409 | Project with sales cannot be deleted                                |
| `PRODUCT_NOT_SELLABLE`    |            409 | Product/variant is missing, invalid, or not assigned to the project |
| `INSUFFICIENT_STOCK`      |            409 | One or more requested items exceed current stock                    |
| `INVALID_PAYMENT_SLIP`    |            409 | Slip object is missing, invalid, or not trusted for checkout        |
| `IDEMPOTENCY_KEY_REUSED`  |            409 | Key was previously used with different checkout content             |
| `INTERNAL_ERROR`          |            500 | Unexpected server failure                                           |

Every operation documents `400`, `401`, `403`, and `500`, plus relevant `404`, `409`, and `413` responses.

## 6. Frontend implementation notes

- Treat all IDs and order numbers as opaque values.
- Do not calculate authoritative checkout totals locally; display the quotation response.
- Requote after cart edits and before enabling confirmation.
- Preserve draft cart state when the checkout dialog closes with `X`.
- Clear cart state after explicit cancellation or successful checkout.
- Generate one idempotency key per confirmation attempt and reuse it for transport retries.
- On `INSUFFICIENT_STOCK`, update affected quantities from `details.items` and let staff amend the cart.
- Disable configuration mutations when project status is `COMPLETED`.
- Use the current order-history filters for the export request.

## 7. Review checklist

Product/PM review:

- [ ] Project status and inclusive Bangkok date boundaries are correct.
- [ ] Projects with orders must be preserved.
- [ ] Configuration may change while ACTIVE but not after completion.
- [ ] Promotion price may equal original price, resulting in zero discount.
- [ ] Equal-discount promotion tie-breaking is acceptable.
- [ ] Buyer gender and optional demographic fields match reporting needs.
- [ ] `no_change` retained-cash behavior is correct.
- [ ] POS orders should immediately be `COMPLETED`.
- [ ] One export row per order item is acceptable.

Frontend review:

- [ ] Product candidate and selection-preview shapes support cross-page selection.
- [ ] Full-replacement product save supports Save/Cancel UX.
- [ ] POS catalogue provides everything needed without additional product calls.
- [ ] Quote and checkout payloads support QR and cash flows.
- [ ] Error `code` and `details` are sufficient for field and stock-conflict UI.
- [ ] Order list/detail/export filters match the planned screens.

Backend review:

- [ ] Route and DTO shapes build cleanly on the reduced core API.
- [ ] Variantless-product representation is compatible with the final schema.
- [ ] Project price and promotion calculations can share one pricing service.
- [ ] Idempotency semantics and transaction boundaries are implementable.
- [ ] Slip object ownership and validation can be enforced.
- [ ] List queries can avoid N+1 behavior.
- [ ] Migration and indexes can support filters, category order, checkout locks, and order history.

## 8. Contract validation status

The canonical OpenAPI contains 23 planned Project/POS operation IDs, explicit Admin security, request/response examples, and documented error responses. These operations remain visible in `/docs` under tags labeled `PLANNED`. Backend implementation and Bruno requests are intentionally deferred until Project/POS implementation begins.
