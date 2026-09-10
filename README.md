# Intania Shop API

Admin-only backend for authentication, product catalogue management, inventory, and uploads. The previous customer storefront domains have been removed. Project/POS is designed contract-first in the OpenAPI document and will be implemented next.

## Current scope

The server currently implements 22 method/path combinations:

- 4 service and documentation routes
- 3 Google OAuth routes
- 9 product and variant routes
- 4 inventory routes
- 2 upload routes

All catalogue, inventory, and upload operations require an authenticated `ADMIN` JWT. `USER` accounts can complete Google OAuth but cannot access business APIs.

The interactive `/docs` page also shows the planned Project/POS operations. Their tags are explicitly labeled `PLANNED`; they are API contracts for frontend development and are not registered by the current server.

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

There is one clean baseline migration. It creates only:

- `users`
- `products`
- `variants`
- `stock_transactions`

It also creates only `user_role`, `product_status`, `stock_transaction_type`, and `product_type` enums. Migration history from the storefront backend is intentionally unsupported.

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

Stock adjustments lock the affected row and persist the transaction atomically. Inventory records retain reason, notes, references, actor, and confirmation image. The `ORDER` transaction type is reserved for future Project/POS checkout.

### Project Promotions

| Method | Path | Authorization | Description |
| --- | --- | --- | --- |
| `GET` | `/projects/{project_id}/promotions` | Authenticated | List project Promotions with current item data and calculated totals |
| `POST` | `/projects/{project_id}/promotions` | `ADMIN` | Create a Promotion |
| `GET` | `/projects/{project_id}/promotions/{promotion_id}` | Authenticated | Get one hydrated Promotion |
| `PUT` | `/projects/{project_id}/promotions/{promotion_id}` | `ADMIN` | Replace the complete Promotion and item set atomically |
| `DELETE` | `/projects/{project_id}/promotions/{promotion_id}` | `ADMIN` | Delete a Promotion |

Promotion prices are fixed THB amounts represented as JSON strings with two decimal places. Responses recalculate `original_bundle_price` and `discount` from the current Project Product prices. POS quotations apply the eligible Promotion with the highest fixed-point discount, using the lowest `promotion_id` as the deterministic tie-breaker.

### Admin POS catalogue and quotations

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/projects/{project_id}/pos` | Preview the selected Project catalogue with current stock and project prices |
| `POST` | `/projects/{project_id}/checkout/quote` | Validate an ACTIVE-project Cart and calculate authoritative totals |

Both endpoints require an `ADMIN` bearer token. The catalogue is available for preview in every Project status; `can_checkout` is true only for an `ACTIVE` Project. Quote requests contain only product/variant identities and positive quantities. Prices, stock, Promotions, discounts, and totals are resolved on the server.

## Documentation

- [`docs/openapi.yaml`](docs/openapi.yaml) is the single machine-readable source of truth.
- [`docs/project-pos-api-contract.md`](docs/project-pos-api-contract.md) is reserved for the review companion to the Project/POS contract.
- `/docs` renders implemented Project/POS catalogue and quotation operations alongside planned checkout/order operations.

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
