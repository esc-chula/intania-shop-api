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

Required settings are `DATABASE_URL`, `JWT_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`, `FRONTEND_CALLBACK_URL`, and `GCS_BUCKET`. Local Google Cloud authentication may use `GOOGLE_APPLICATION_CREDENTIALS`; deployed environments should use an attached service account.

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

After Google sign-in, the callback redirects to `FRONTEND_CALLBACK_URL` with a short-lived, single-use `code` query parameter. The frontend must immediately exchange that code with the API to receive a JWT. Send the returned JWT as:

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
| `GET` | `/auth/google/callback` | Complete OAuth and redirect to the configured frontend callback with a one-time code |
| `POST` | `/auth/exchange` | Exchange a one-time login code for a JWT |

### Admin products and variants

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/products` | List products |
| `GET` | `/products/search` | Search products |
| `GET` | `/products/{id}` | Product detail |
| `POST` | `/products` | Create product |
| `PUT` | `/products/{id}` | Update product |
| `DELETE` | `/products/{id}` | Delete product |
| `POST` | `/products/{id}/variants` | Create variant |
| `PUT` | `/variants/{id}` | Update variant |
| `DELETE` | `/variants/{id}` | Delete variant |

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

Stock changes are atomic and recorded in inventory history. Payment slips accept one JPEG, PNG, or WebP image (up to 10 MiB); use the returned `object_key` for QR checkout.

### Project Promotions

| Method | Path | Authorization | Description |
| --- | --- | --- | --- |
| `GET` | `/projects/{project_id}/promotions` | `ADMIN` | List Promotions |
| `POST` | `/projects/{project_id}/promotions` | `ADMIN` | Create a Promotion |
| `GET` | `/projects/{project_id}/promotions/{promotion_id}` | `ADMIN` | Get Promotion |
| `PUT` | `/projects/{project_id}/promotions/{promotion_id}` | `ADMIN` | Replace Promotion |
| `DELETE` | `/projects/{project_id}/promotions/{promotion_id}` | `ADMIN` | Delete a Promotion |

Promotion amounts are two-decimal THB strings. Quotes apply the eligible Promotion with the highest discount.

### Admin POS catalogue and quotations

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/projects/{project_id}/pos` | View Project catalogue |
| `POST` | `/projects/{project_id}/checkout/quote` | Quote a Cart |
| `POST` | `/projects/{project_id}/orders` | Checkout and reduce stock |

All three require `ADMIN`. Catalogue preview works for every Project; quote and checkout require an `ACTIVE` Project and use server-calculated prices and stock.

### Atomic POS checkout

Checkout requires an `ACTIVE` Project and `Idempotency-Key`. It atomically validates stock, creates immutable order snapshots, records payment, and reduces stock; failed checkouts roll back. Quotes reserve nothing. QR checkout uses a previously uploaded slip, while cash must cover the server total. Retrying the same key and payload returns the original order without another stock reduction.

### Admin Project order history

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/projects/{project_id}/orders` | List orders |
| `GET` | `/projects/{project_id}/orders/{order_id}` | Get order detail |
| `GET` | `/projects/{project_id}/orders/export` | Export orders as Excel |

All three require `ADMIN`. List and export support filtering and sorting; orders use checkout-time snapshots. A Project with orders cannot be deleted or moved outside an order date.

## Documentation

- [`docs/openapi.yaml`](docs/openapi.yaml) is the single machine-readable source of truth.
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
