# Intania Shop API

Admin-only backend for authentication, product catalogue management, inventory, uploads, Project administration, Promotions, POS catalogue/quotation/paid checkout, and Project order history APIs (list, detail, export). The previous customer storefront domains have been removed.

## Technology

- Go 1.26.5
- Chi v5 and `net/http`
- PostgreSQL with pgx 5
- Goose database migrations
- Google OAuth 2.0 with PKCE and HS256 JWTs
- Google Cloud Storage
- Structured logging with `log/slog`

## Configuration

Copy the example and configure local credentials:

```bash
cp .env.example .env
```

Required settings are `DATABASE_URL`, `JWT_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`, and `GCS_BUCKET`. Local Google Cloud authentication may use `GOOGLE_APPLICATION_CREDENTIALS`; deployed environments should use an attached service account.

The server validates required configuration at startup. `make run`, `make migrate`, and Air load `.env`; raw Go commands require exported environment variables.

## Clean database setup

Migration history from the storefront backend is intentionally unsupported.

Reset the local database before using this version:

```bash
docker compose down -v
docker compose up -d db
make migrate
```

Then start the API:

```bash
make run
```

For hot reload, use `make dev` after installing Air.

## Local frontend seed data

For a repeatable local catalogue and POS fixture, run this after migrations:

```bash
make seed
```

This command runs only against the Docker `db` service. It resets local
catalogue, inventory, Project, Promotion, payment-slip, and POS-order data,
but preserves OAuth `users`. Do not use it against production or any database
whose shop data must be retained.

The fixture creates an ACTIVE `Intania Music Fest 2026` Project, five themed
products (including a Polo with M/L variants), positive initial stock and
stock history, Project selling prices, and the `Swagged Out Set` Promotion.
It deliberately has no product images, GCS objects, user accounts, or sample
orders. It prepares consistent frontend demo data; use the Bruno release suite
to exercise API routes, authorization failures, checkout rollback, and
external GCS flows.

## Authentication

Google OAuth is the only account creation and login flow. Open:

```text
http://localhost:8080/auth/google/redirect
```

After Google sign-in, the callback returns a JWT. Send it as:

```http
Authorization: Bearer <token>
```

New accounts have the `USER` role unless promoted to `ADMIN`. Only administrators can access products, variants, inventory, and uploads.

## Implemented API

### Public service and authentication

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/` | Service name |
| `GET` | `/health` | Database-backed health check |
| `GET` | `/openapi.yaml` | Canonical OpenAPI contract |
| `GET` | `/docs` | Interactive API reference |
| `GET` | `/auth/google` | Create an OAuth authorization URL |
| `GET` | `/auth/google/redirect` | Redirect to Google OAuth |
| `GET` | `/auth/google/callback` | Complete OAuth and return a JWT |

### Admin products and variants

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/products` | Paginated list; optionally include variants |
| `GET` | `/products/search` | Paginated name search |
| `GET` | `/products/{id}` | Product detail |
| `POST` | `/products` | Create product |
| `PUT` | `/products/{id}` | Update product |
| `DELETE` | `/products/{id}` | Delete product |
| `POST` | `/products/{id}/variants` | Create variant |
| `PUT` | `/variants/{id}` | Update variant |
| `DELETE` | `/variants/{id}` | Delete variant |

The clean product model retains name, description, price, status, category, stock, images, product type, SKU, product code, timestamps, and variants.

### Admin inventory and uploads

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/products/{id}/stock/adjust` | Adjust product or variant stock |
| `GET` | `/products/{id}/stock/transactions` | Product stock history |
| `GET` | `/variants/{id}/stock/transactions` | Variant stock history |
| `GET` | `/stock/transactions` | All stock history |
| `POST` | `/upload/product-images` | Upload product images |
| `POST` | `/stock/upload-proof-images` | Upload a stock confirmation image |
| `POST` | `/upload/payment-slips` | Upload one trusted QR payment slip |

Stock adjustments lock the affected row and persist the transaction atomically. Inventory records retain reason, notes, references, actor, and confirmation image. POS checkout writes the `ORDER` transaction type, referencing the order it belongs to.

Payment slip uploads accept exactly one JPEG, PNG, or WebP image of at most 10 MiB in the `file` field. The content type is detected from the file itself, and the returned `object_key` is the only slip reference checkout accepts.

### Project Promotions

| Method | Path | Authorization | Description |
| --- | --- | --- | --- |
| `GET` | `/projects/{project_id}/promotions` | `ADMIN` | List project Promotions with current item data and calculated totals |
| `POST` | `/projects/{project_id}/promotions` | `ADMIN` | Create a Promotion |
| `GET` | `/projects/{project_id}/promotions/{promotion_id}` | `ADMIN` | Get one hydrated Promotion |
| `PUT` | `/projects/{project_id}/promotions/{promotion_id}` | `ADMIN` | Replace the complete Promotion and item set atomically |
| `DELETE` | `/projects/{project_id}/promotions/{promotion_id}` | `ADMIN` | Delete a Promotion |

Promotion prices are fixed THB amounts represented as JSON strings with two decimal places. Responses recalculate `original_bundle_price` and `discount` from the current Project Product prices. POS quotations apply the eligible Promotion with the highest fixed-point discount, using the lowest `promotion_id` as the deterministic tie-breaker.

### Admin POS catalogue and quotations

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/projects/{project_id}/pos` | Preview the selected Project catalogue with current stock and project prices |
| `POST` | `/projects/{project_id}/checkout/quote` | Validate an ACTIVE-project Cart and calculate authoritative totals |
| `POST` | `/projects/{project_id}/orders` | Create a paid order and reduce stock atomically |

All three endpoints require an `ADMIN` bearer token. The catalogue is available for preview in every Project status; `can_checkout` is true only for an `ACTIVE` Project. Quote and checkout requests contain only product/variant identities and positive quantities. Prices, stock, Promotions, discounts, and totals are resolved on the server.

### Atomic POS checkout

`POST /projects/{project_id}/orders` requires an `Idempotency-Key` header and an `ACTIVE` Project. One transaction locks the Project and the stock rows of every requested item, revalidates the selection, prices, Promotions, and stock, then writes the order, its item, payment, and Promotion snapshots, and one `ORDER` stock transaction per line. Any rejection rolls the whole set back, so a failed checkout leaves no order, payment, snapshot, or stock change behind.

- A quotation is a preview and reserves nothing; only checkout reduces stock.
- Snapshots keep the name, variant text, image, and prices that applied at payment time, so later catalogue or Promotion edits never rewrite a paid order.
- `QR_CODE` payments must reference a `slip_object_key` returned by `POST /upload/payment-slips`, and each slip can back only one order.
- `REAL_MONEY` payments must cover the server-calculated `net_total`. When `no_change` is true, any excess is retained rather than returned as change.
- Retrying with the same `Idempotency-Key` and the same payload returns the stored order with `Idempotency-Replayed: true` and `200`, without reducing stock again. The same key with a different payload is rejected with `IDEMPOTENCY_KEY_REUSED`.

### Admin Project order history

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/projects/{project_id}/orders` | List Project POS orders with filtering, sorting, and pagination |
| `GET` | `/projects/{project_id}/orders/{order_id}` | Get one immutable order, with its item, buyer, staff, payment, and applied-promotion snapshots |
| `GET` | `/projects/{project_id}/orders/export` | Export the filtered, sorted order history as an Excel workbook, one row per order item |

All three require an `ADMIN` bearer token. `order_number`, `payment_method`, `staff_id`, `created_from`, and `created_to` filter the list before pagination; `sort_by` (`order_number` or `net_total`) and `sort_order` always carry a fixed `order_id` tie-break, so paging never reorders across pages. `created_from`/`created_to` default to 00:00 `Asia/Bangkok` on the Project's start date through the current time. Export uses the same filters and sort as the list, without pagination.

Every returned order reads the buyer, staff, payment, item, and applied-promotion data recorded at checkout; it is never rejoined against the current Product, User, or Promotion tables, so later edits to any of those never change order history. A Project that has orders cannot be deleted, and its sale dates cannot be changed to exclude an existing order's date (`409 PROJECT_CONFLICT`).

Order history reads immutable snapshots created by checkout; manual request coverage is documented in [`docs/manual-tests/BE-009-orders.http`](docs/manual-tests/BE-009-orders.http).

## Documentation

- [`docs/openapi.yaml`](docs/openapi.yaml) is the single machine-readable source of truth.
- [`docs/project-pos-api-contract.md`](docs/project-pos-api-contract.md) is reserved for the review companion to the Project/POS contract.
- `/docs` renders implemented Project/POS and order history operations.

## Responses

Successful JSON responses use:

```json
{"success": true, "data": {}}
```

Legacy endpoints retain the existing human-readable shape:

```json
{"success": false, "error": "Product not found"}
```

Project/POS errors use stable `code`, optional `details`, and `request_id` fields. Stock conflicts include the affected Cart lines in `details.items`.

## Quality checks

```bash
make check
```

Or run checks individually:

```bash
make fmt
make vet
make test
make lint
make build
make docs-check
```

Integration tests require an isolated, disposable PostgreSQL database:

```bash
TEST_DATABASE_URL="postgres://postgres:postgres@localhost:55433/intania_shop_test?sslmode=disable" make test-integration
```

## Project structure

```text
cmd/                    CLI commands: serve and migrate
internal/
├── auth/               Google OAuth and PKCE
├── config/             Environment validation
├── database/           PostgreSQL pool
├── handlers/           HTTP transport
├── middlewares/        Authentication, roles, logging, and recovery
├── migrations/         Clean Goose baseline
├── models/             Core API models
├── repositories/       PostgreSQL access and transactions
├── server/             Routes, wiring, and lifecycle
├── storage/            Google Cloud Storage
└── usecases/           Business rules
docs/                   OpenAPI and Project/POS contract companion
```
